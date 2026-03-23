package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/research"
)

// GatherResult holds the outcome of a GATHER action.
type GatherResult struct {
	ResourceType string
	Amount       int
	PayloadEN    string
	PayloadDE    string
}

// processGather handles the GATHER action.
func processGather(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64) GatherResult {
	// 1. Get agent's ship location (planet_id, system_id).
	var systemID string
	var planetID *string
	err := pool.QueryRow(ctx,
		`SELECT system_id, planet_id FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&systemID, &planetID)
	if err != nil {
		return GatherResult{
			PayloadEN: "Gather failed: ship not found.",
			PayloadDE: "Sammeln fehlgeschlagen: Schiff nicht gefunden.",
		}
	}

	// 2. Must be landed on a planet.
	if planetID == nil {
		return GatherResult{
			PayloadEN: "Not landed on a planet.",
			PayloadDE: "Nicht auf einem Planeten gelandet.",
		}
	}

	// 3. Get resource_nodes from the planet.
	var resourceNodesRaw []byte
	err = pool.QueryRow(ctx,
		`SELECT resource_nodes FROM planets WHERE id = $1`,
		*planetID,
	).Scan(&resourceNodesRaw)
	if err != nil {
		return GatherResult{
			PayloadEN: "Gather failed: planet data unavailable.",
			PayloadDE: "Sammeln fehlgeschlagen: Planetendaten nicht verfügbar.",
		}
	}

	var resourceNodes []string
	if err := json.Unmarshal(resourceNodesRaw, &resourceNodes); err != nil || len(resourceNodes) == 0 {
		return GatherResult{
			PayloadEN: "No harvestable resources found on this planet.",
			PayloadDE: "Keine abbaubaren Ressourcen auf diesem Planeten gefunden.",
		}
	}

	// 4. Pick first resource (deterministic).
	resourceType := resourceNodes[0]

	// 5. Base amount: 10 + (tickNumber % 21) — deterministic 10-30.
	// Apply research gather bonus on top.
	var researchRaw []byte
	_ = pool.QueryRow(ctx,
		`SELECT research FROM agents WHERE id = $1`, agentID,
	).Scan(&researchRaw)
	var completedResearch []string
	_ = json.Unmarshal(researchRaw, &completedResearch)
	gatherBonus := research.GatherBonus(completedResearch)

	base := 10 + int(tickNumber%21)
	amount := base + int(float64(base)*gatherBonus)

	// 6. Upsert into inventories.
	_, err = pool.Exec(ctx,
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type) DO UPDATE
		   SET quantity = inventories.quantity + EXCLUDED.quantity`,
		agentID, resourceType, amount,
	)
	if err != nil {
		return GatherResult{
			PayloadEN: fmt.Sprintf("Gather failed: could not update inventory: %v", err),
			PayloadDE: fmt.Sprintf("Sammeln fehlgeschlagen: Inventar konnte nicht aktualisiert werden: %v", err),
		}
	}

	// 7. Award 5 XP.
	_, _ = pool.Exec(ctx,
		`UPDATE agents SET experience = experience + 5 WHERE id = $1`,
		agentID,
	)

	// 8. Get planet name for the payload.
	var planetName string
	_ = pool.QueryRow(ctx,
		`SELECT name FROM planets WHERE id = $1`,
		*planetID,
	).Scan(&planetName)
	if planetName == "" {
		planetName = *planetID
	}

	return GatherResult{
		ResourceType: resourceType,
		Amount:       amount,
		PayloadEN:    fmt.Sprintf("Gathered %d units of %s from %s.", amount, resourceType, planetName),
		PayloadDE:    fmt.Sprintf("%d Einheiten %s von %s gesammelt.", amount, resourceType, planetName),
	}
}
