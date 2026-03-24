package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/mining"
)

type skillResult struct {
	PayloadEN string
	PayloadDE string
}

// processUseSkill handles the USE_SKILL action.
// Parameters: {"skill_id": "overcharge_drill"}
func processUseSkill(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage, tickNumber int64) skillResult {
	var p struct {
		SkillID string `json:"skill_id"`
	}
	_ = json.Unmarshal(params, &p)

	sk, ok := mining.SkillRegistry[p.SkillID]
	if !ok {
		return skillResult{
			PayloadEN: fmt.Sprintf("Unknown skill: %s", p.SkillID),
			PayloadDE: fmt.Sprintf("Unbekannter Skill: %s", p.SkillID),
		}
	}

	// Upsert skill row (creates at level 1 if not exists) and load current state.
	_, err := pool.Exec(ctx,
		`INSERT INTO agent_skills (agent_id, skill_id, level, xp, cooldown_expires_tick)
		 VALUES ($1, $2, 1, 0, 0)
		 ON CONFLICT (agent_id, skill_id) DO NOTHING`,
		agentID, p.SkillID,
	)
	if err != nil {
		slog.Error("processUseSkill: upsert skill", "agent", agentID, "err", err)
	}

	var level int
	var xp int
	var cooldownExpires int64
	if err := pool.QueryRow(ctx,
		`SELECT level, xp, cooldown_expires_tick FROM agent_skills WHERE agent_id = $1 AND skill_id = $2`,
		agentID, p.SkillID,
	).Scan(&level, &xp, &cooldownExpires); err != nil {
		return skillResult{PayloadEN: "Skill lookup failed.", PayloadDE: "Skill-Abfrage fehlgeschlagen."}
	}

	// Check cooldown.
	if tickNumber <= cooldownExpires {
		remaining := cooldownExpires - tickNumber + 1
		return skillResult{
			PayloadEN: fmt.Sprintf("%s is on cooldown for %d more tick(s).", sk.Name, remaining),
			PayloadDE: fmt.Sprintf("%s ist noch %d Tick(s) auf Abklingzeit.", sk.NameDE, remaining),
		}
	}

	// Dispatch to skill-specific logic.
	var result skillResult
	switch p.SkillID {
	case "overcharge_drill":
		result = activateOverchargeDrill(ctx, pool, agentID, level, tickNumber)
	case "deep_survey":
		result = activateDeepSurvey(ctx, pool, agentID, level, tickNumber)
	case "cargo_compress":
		result = activateCargoCompress(ctx, pool, agentID, level, tickNumber)
	case "scavenge":
		result = activateScavenge(ctx, pool, agentID, level, tickNumber)
	case "emergency_jettison":
		result = activateEmergencyJettison(ctx, pool, agentID, tickNumber)
	default:
		return skillResult{PayloadEN: "Skill not yet implemented.", PayloadDE: "Skill noch nicht implementiert."}
	}

	// Apply cooldown and award XP.
	newXP := xp + sk.XPPerUse
	newLevel := mining.LevelForXP(sk, newXP)
	newCooldown := tickNumber + int64(sk.CooldownTicks)

	_, _ = pool.Exec(ctx,
		`UPDATE agent_skills SET level = $1, xp = $2, cooldown_expires_tick = $3
		 WHERE agent_id = $4 AND skill_id = $5`,
		newLevel, newXP, newCooldown, agentID, p.SkillID,
	)

	if newLevel > level {
		result.PayloadEN += fmt.Sprintf(" %s leveled up to L%d!", sk.Name, newLevel)
		result.PayloadDE += fmt.Sprintf(" %s auf Level %d aufgestiegen!", sk.NameDE, newLevel)
	}

	return result
}

func activateOverchargeDrill(ctx context.Context, pool *pgxpool.Pool, agentID string, level int, tickNumber int64) skillResult {
	mult := mining.OverchargeDrillMultiplier(level)
	// Boost lasts until end of current tick's processing window + 1 (the next MINE call in this tick or next tick).
	expiresAt := tickNumber + 2
	_, err := pool.Exec(ctx,
		`INSERT INTO skill_boosts (agent_id, skill_id, magnitude, expires_at_tick)
		 VALUES ($1, 'overcharge_drill', $2, $3)
		 ON CONFLICT (agent_id, skill_id) DO UPDATE SET magnitude = EXCLUDED.magnitude, expires_at_tick = EXCLUDED.expires_at_tick`,
		agentID, mult, expiresAt,
	)
	if err != nil {
		slog.Error("activateOverchargeDrill: insert boost", "agent", agentID, "err", err)
	}
	return skillResult{
		PayloadEN: fmt.Sprintf("Overcharge Drill L%d activated: next MINE yields %.0f%% extra.", level, (mult-1)*100),
		PayloadDE: fmt.Sprintf("Überladener Bohrer L%d aktiviert: nächstes MINE bringt %.0f%% mehr Ertrag.", level, (mult-1)*100),
	}
}

func activateDeepSurvey(ctx context.Context, pool *pgxpool.Pool, agentID string, level int, tickNumber int64) skillResult {
	duration := mining.DeepSurveyDuration(level)

	// Get current galaxy.
	var galaxyID string
	_ = pool.QueryRow(ctx,
		`SELECT galaxy_id FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&galaxyID)

	// Fetch all distinct system_ids in the galaxy that have mining nodes.
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT system_id FROM mining_nodes
		 WHERE system_id IN (SELECT system_id FROM planets WHERE galaxy_id = $1)`,
		galaxyID,
	)
	if err != nil {
		return skillResult{PayloadEN: "Deep Survey failed.", PayloadDE: "Tiefenscan fehlgeschlagen."}
	}
	var systemIDs []string
	for rows.Next() {
		var sid string
		if rows.Scan(&sid) == nil {
			systemIDs = append(systemIDs, sid)
		}
	}
	rows.Close()

	expiresAt := tickNumber + duration
	surveyed := 0
	for _, sid := range systemIDs {
		_, _ = pool.Exec(ctx,
			`INSERT INTO surveys (agent_id, system_id, surveyed_at_tick, expires_at_tick)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (agent_id, system_id) DO UPDATE
			   SET surveyed_at_tick = EXCLUDED.surveyed_at_tick,
			       expires_at_tick  = EXCLUDED.expires_at_tick`,
			agentID, sid, tickNumber, expiresAt,
		)
		surveyed++
	}

	return skillResult{
		PayloadEN: fmt.Sprintf("Deep Survey L%d: %d system(s) in %s scanned. Data valid for %d ticks.", level, surveyed, galaxyID, duration),
		PayloadDE: fmt.Sprintf("Tiefenscan L%d: %d System(e) in %s gescannt. Daten gültig für %d Ticks.", level, surveyed, galaxyID, duration),
	}
}

func activateCargoCompress(ctx context.Context, pool *pgxpool.Pool, agentID string, level int, tickNumber int64) skillResult {
	bonus := mining.CargoCompressBonus(level)
	duration := int64(level * 3)
	expiresAt := tickNumber + duration

	_, err := pool.Exec(ctx,
		`INSERT INTO skill_boosts (agent_id, skill_id, magnitude, expires_at_tick)
		 VALUES ($1, 'cargo_compress', $2, $3)
		 ON CONFLICT (agent_id, skill_id) DO UPDATE SET magnitude = EXCLUDED.magnitude, expires_at_tick = EXCLUDED.expires_at_tick`,
		agentID, float64(bonus), expiresAt,
	)
	if err != nil {
		slog.Error("activateCargoCompress: insert boost", "agent", agentID, "err", err)
	}
	return skillResult{
		PayloadEN: fmt.Sprintf("Cargo Compress L%d: +%d cargo capacity for %d ticks.", level, bonus, duration),
		PayloadDE: fmt.Sprintf("Laderaumkomprimierung L%d: +%d Ladekapazität für %d Ticks.", level, bonus, duration),
	}
}

func activateScavenge(ctx context.Context, pool *pgxpool.Pool, agentID string, level int, tickNumber int64) skillResult {
	minY, maxY := mining.ScavengeYield(level)

	// Get current system and its mining nodes.
	var systemID string
	_ = pool.QueryRow(ctx,
		`SELECT system_id FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&systemID)

	// Pick a random resource from nodes in the system.
	rows, err := pool.Query(ctx,
		`SELECT resource_type FROM mining_nodes WHERE system_id = $1`,
		systemID,
	)
	if err != nil {
		return skillResult{PayloadEN: "Scavenge failed.", PayloadDE: "Plündern fehlgeschlagen."}
	}
	var resources []string
	for rows.Next() {
		var r string
		if rows.Scan(&r) == nil {
			resources = append(resources, r)
		}
	}
	rows.Close()

	if len(resources) == 0 {
		return skillResult{
			PayloadEN: "Nothing worth scavenging in this system.",
			PayloadDE: "Nichts Verwertbares in diesem System zum Plündern.",
		}
	}

	rng := rand.New(rand.NewSource(tickNumber + fnvHash(agentID)))
	resourceType := resources[rng.Intn(len(resources))]
	amount := minY + rng.Intn(maxY-minY+1)

	_, _ = pool.Exec(ctx,
		`INSERT INTO inventories (agent_id, resource_type, quantity) VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type) DO UPDATE SET quantity = inventories.quantity + EXCLUDED.quantity`,
		agentID, resourceType, amount,
	)
	_, _ = pool.Exec(ctx, `UPDATE agents SET experience = experience + 5 WHERE id = $1`, agentID)

	return skillResult{
		PayloadEN: fmt.Sprintf("Scavenge L%d: salvaged %d units of %s.", level, amount, resourceType),
		PayloadDE: fmt.Sprintf("Plündern L%d: %d Einheiten %s geborgen.", level, amount, resourceType),
	}
}

func activateEmergencyJettison(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64) skillResult {
	// Load entire inventory.
	type invEntry struct {
		ResourceType string
		Quantity     int
	}
	rows, err := pool.Query(ctx, `SELECT resource_type, quantity FROM inventories WHERE agent_id = $1 AND quantity > 0`, agentID)
	if err != nil {
		return skillResult{PayloadEN: "Jettison failed.", PayloadDE: "Notabwurf fehlgeschlagen."}
	}
	var inv []invEntry
	for rows.Next() {
		var e invEntry
		if rows.Scan(&e.ResourceType, &e.Quantity) == nil {
			inv = append(inv, e)
		}
	}
	rows.Close()

	if len(inv) == 0 {
		return skillResult{
			PayloadEN: "Cargo hold is already empty.",
			PayloadDE: "Laderaum ist bereits leer.",
		}
	}

	// Base market prices for compensation (25% of base value).
	basePrices := map[string]int{
		"naquadah":    50,
		"trinium":     30,
		"naquadriah":  150,
		"ancient_tech": 200,
	}

	totalCredits := 0
	totalUnits := 0
	for _, e := range inv {
		price := basePrices[e.ResourceType]
		credits := int(float64(e.Quantity*price) * 0.25)
		totalCredits += credits
		totalUnits += e.Quantity
	}

	// Clear inventory and award partial compensation.
	_, _ = pool.Exec(ctx, `DELETE FROM inventories WHERE agent_id = $1`, agentID)
	if totalCredits > 0 {
		_, _ = pool.Exec(ctx,
			`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
			totalCredits, agentID,
		)
	}

	return skillResult{
		PayloadEN: fmt.Sprintf("Emergency Jettison: %d units discarded, +%d credits compensation (25%% salvage).", totalUnits, totalCredits),
		PayloadDE: fmt.Sprintf("Notabwurf: %d Einheiten abgeworfen, +%d Credits Entschädigung (25%% Bergungswert).", totalUnits, totalCredits),
	}
}
