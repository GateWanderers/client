package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/mining"
	"gatewanderers/server/internal/research"
)

type mineResult struct {
	ResourceType string
	Amount       int
	PayloadEN    string
	PayloadDE    string
}

// processMine handles the MINE action — targeted extraction from a mining node.
// Parameters: {"resource_type": "naquadah"}
func processMine(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage, tickNumber int64) mineResult {
	var p struct {
		ResourceType string `json:"resource_type"`
	}
	_ = json.Unmarshal(params, &p)
	if p.ResourceType == "" {
		return mineResult{
			PayloadEN: "MINE requires a resource_type parameter (e.g. naquadah, trinium).",
			PayloadDE: "MINE erfordert einen resource_type-Parameter (z.B. naquadah, trinium).",
		}
	}

	// Load ship location and cargo capacity.
	var systemID string
	var planetID *string
	var cargoCapacity int
	err := pool.QueryRow(ctx,
		`SELECT system_id, planet_id, cargo_capacity
		 FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&systemID, &planetID, &cargoCapacity)
	if err != nil {
		return mineResult{PayloadEN: "Mine failed: ship not found.", PayloadDE: "Mine fehlgeschlagen: Schiff nicht gefunden."}
	}
	if planetID == nil {
		return mineResult{
			PayloadEN: "You must be landed on a planet to mine.",
			PayloadDE: "Du musst auf einem Planeten gelandet sein, um zu minen.",
		}
	}

	// Load completed research for bonuses.
	var researchRaw []byte
	_ = pool.QueryRow(ctx, `SELECT research FROM agents WHERE id = $1`, agentID).Scan(&researchRaw)
	var completedResearch []string
	_ = json.Unmarshal(researchRaw, &completedResearch)

	// Compute effective cargo capacity (base + research bonus + skill boost).
	researchCargoBonux := research.CargoBonus(completedResearch)
	var skillBoostMag float64
	_ = pool.QueryRow(ctx,
		`SELECT magnitude FROM skill_boosts WHERE agent_id = $1 AND skill_id = 'cargo_compress' AND expires_at_tick >= $2`,
		agentID, tickNumber,
	).Scan(&skillBoostMag)
	effectiveCargo := cargoCapacity + researchCargoBonux + int(skillBoostMag)

	// Check current cargo usage (sum of all inventories).
	var cargoUsed int
	_ = pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(quantity), 0) FROM inventories WHERE agent_id = $1`,
		agentID,
	).Scan(&cargoUsed)

	if cargoUsed >= effectiveCargo {
		return mineResult{
			PayloadEN: fmt.Sprintf("Cargo hold full (%d/%d). Sell or jettison resources first.", cargoUsed, effectiveCargo),
			PayloadDE: fmt.Sprintf("Laderaum voll (%d/%d). Verkaufe oder werfe zuerst Ressourcen ab.", cargoUsed, effectiveCargo),
		}
	}

	// Find the mining node for this system and resource type.
	var nodeID string
	var richness string
	var currentReserves int
	err = pool.QueryRow(ctx,
		`SELECT id, richness, current_reserves FROM mining_nodes WHERE system_id = $1 AND resource_type = $2`,
		systemID, p.ResourceType,
	).Scan(&nodeID, &richness, &currentReserves)
	if err != nil {
		return mineResult{
			PayloadEN: fmt.Sprintf("No %s deposit found in this system. Try SURVEY first.", p.ResourceType),
			PayloadDE: fmt.Sprintf("Kein %s-Vorkommen in diesem System gefunden. Versuche zuerst SURVEY.", p.ResourceType),
		}
	}
	if currentReserves <= 0 {
		return mineResult{
			PayloadEN: fmt.Sprintf("The %s node is exhausted. Wait for regeneration.", p.ResourceType),
			PayloadDE: fmt.Sprintf("Das %s-Vorkommen ist erschöpft. Warte auf Regeneration.", p.ResourceType),
		}
	}

	// Load richness config.
	nodeConfig, ok := mining.Configs[mining.Richness(richness)]
	if !ok {
		nodeConfig = mining.Configs[mining.RichnessNormal]
	}

	// Base yield from richness tier (deterministic RNG per agent+resource+tick).
	rng := rand.New(rand.NewSource(tickNumber + fnvHash(agentID+p.ResourceType)))
	yieldRange := nodeConfig.YieldMax - nodeConfig.YieldMin
	baseYield := nodeConfig.YieldMin + rng.Intn(yieldRange+1)

	// Apply research mine bonus.
	mineBonus := research.MineBonus(completedResearch, p.ResourceType)
	yield := float64(baseYield) * (1.0 + mineBonus)

	// strip_mining: 2× yield, 2× depletion.
	stripMining := research.HasStripMining(completedResearch)
	depletionMultiplier := 1
	if stripMining {
		yield *= 2
		depletionMultiplier = 2
	}

	// overcharge_drill skill boost: check for active boost and consume it.
	var overchargeMag float64
	boostErr := pool.QueryRow(ctx,
		`DELETE FROM skill_boosts WHERE agent_id = $1 AND skill_id = 'overcharge_drill' AND expires_at_tick >= $2
		 RETURNING magnitude`,
		agentID, tickNumber,
	).Scan(&overchargeMag)
	if boostErr == nil && overchargeMag > 0 {
		yield *= overchargeMag
	}

	// Cap yield to remaining reserves and available cargo space.
	finalYield := int(yield)
	if finalYield <= 0 {
		finalYield = 1
	}
	maxByReserves := currentReserves
	maxByCargo := effectiveCargo - cargoUsed
	if finalYield > maxByReserves {
		finalYield = maxByReserves
	}
	if finalYield > maxByCargo {
		finalYield = maxByCargo
	}

	depletion := finalYield * depletionMultiplier
	if depletion > currentReserves {
		depletion = currentReserves
	}

	// Deduct node reserves.
	_, err = pool.Exec(ctx,
		`UPDATE mining_nodes
		 SET current_reserves = GREATEST(0, current_reserves - $1), last_mined_tick = $2
		 WHERE id = $3`,
		depletion, tickNumber, nodeID,
	)
	if err != nil {
		slog.Error("processMine: deduct reserves", "agent", agentID, "err", err)
		return mineResult{PayloadEN: "Mine failed: database error.", PayloadDE: "Mine fehlgeschlagen: Datenbankfehler."}
	}

	// Add to inventory.
	_, err = pool.Exec(ctx,
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type) DO UPDATE SET quantity = inventories.quantity + EXCLUDED.quantity`,
		agentID, p.ResourceType, finalYield,
	)
	if err != nil {
		return mineResult{PayloadEN: "Mine failed: inventory update error.", PayloadDE: "Mine fehlgeschlagen: Inventar konnte nicht aktualisiert werden."}
	}

	// Award XP based on richness.
	_, _ = pool.Exec(ctx,
		`UPDATE agents SET experience = experience + $1 WHERE id = $2`,
		nodeConfig.XPReward, agentID,
	)

	// Get planet name for the message.
	var planetName string
	_ = pool.QueryRow(ctx, `SELECT name FROM planets WHERE id = $1`, *planetID).Scan(&planetName)
	if planetName == "" {
		planetName = systemID
	}

	reservesAfter := currentReserves - depletion
	if reservesAfter < 0 {
		reservesAfter = 0
	}

	return mineResult{
		ResourceType: p.ResourceType,
		Amount:       finalYield,
		PayloadEN: fmt.Sprintf("Mined %d units of %s from %s [%s node, %d reserves remaining].",
			finalYield, p.ResourceType, planetName, richness, reservesAfter),
		PayloadDE: fmt.Sprintf("%d Einheiten %s aus %s abgebaut [%s-Vorkommen, %d Reserven verbleibend].",
			finalYield, p.ResourceType, planetName, richness, reservesAfter),
	}
}
