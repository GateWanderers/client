package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/research"
)

// ResearchStartResult holds the outcome of a RESEARCH action.
type ResearchStartResult struct {
	TechID        string
	TechName      string
	TicksRequired int
	CompletesAt   int64
	PayloadEN     string
	PayloadDE     string
}

type researchParams struct {
	TechID string `json:"tech_id"`
}

// processResearch handles a RESEARCH action.
// Parameters: {"tech_id": "shield_tech"}
func processResearch(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage, tickNumber int64) ResearchStartResult {
	// 1. Parse parameters and get tech from registry.
	var p researchParams
	if err := json.Unmarshal(params, &p); err != nil || p.TechID == "" {
		return ResearchStartResult{
			PayloadEN: "Research failed: invalid parameters.",
			PayloadDE: "Forschung fehlgeschlagen: ungültige Parameter.",
		}
	}

	tech, ok := research.Get(p.TechID)
	if !ok {
		return ResearchStartResult{
			PayloadEN: fmt.Sprintf("Research failed: unknown tech '%s'.", p.TechID),
			PayloadDE: fmt.Sprintf("Forschung fehlgeschlagen: unbekannte Technologie '%s'.", p.TechID),
		}
	}

	// 2. Get agent faction and research JSONB.
	var faction string
	var researchRaw []byte
	err := pool.QueryRow(ctx,
		`SELECT faction, research FROM agents WHERE id = $1`,
		agentID,
	).Scan(&faction, &researchRaw)
	if err != nil {
		return ResearchStartResult{
			PayloadEN: "Research failed: agent not found.",
			PayloadDE: "Forschung fehlgeschlagen: Agent nicht gefunden.",
		}
	}

	var completedList []string
	if err := json.Unmarshal(researchRaw, &completedList); err != nil {
		completedList = []string{}
	}

	// 3. Check faction compatibility and prerequisites.
	if ok, reason := research.CanResearch(tech, faction, completedList); !ok {
		return ResearchStartResult{
			PayloadEN: fmt.Sprintf("Research failed: %s.", reason),
			PayloadDE: fmt.Sprintf("Forschung fehlgeschlagen: %s.", reason),
		}
	}

	// 4. Check agent not already researching.
	var existingTechID string
	err = pool.QueryRow(ctx,
		`SELECT tech_id FROM research_queue WHERE agent_id = $1`,
		agentID,
	).Scan(&existingTechID)
	if err == nil {
		// Row found — agent is already researching something.
		return ResearchStartResult{
			PayloadEN: fmt.Sprintf("Research failed: already researching '%s'.", existingTechID),
			PayloadDE: fmt.Sprintf("Forschung fehlgeschlagen: erforscht bereits '%s'.", existingTechID),
		}
	}

	// 5. Check inventory has enough resources for all cost items.
	for _, cost := range tech.Cost {
		var qty int
		err := pool.QueryRow(ctx,
			`SELECT quantity FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
			agentID, cost.Type,
		).Scan(&qty)
		if err != nil || qty < cost.Amount {
			return ResearchStartResult{
				PayloadEN: fmt.Sprintf("Research failed: insufficient %s (need %d).", cost.Type, cost.Amount),
				PayloadDE: fmt.Sprintf("Forschung fehlgeschlagen: unzureichend %s (benötigt %d).", cost.Type, cost.Amount),
			}
		}
	}

	// 6. All checks passed — begin transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return ResearchStartResult{
			PayloadEN: "Research failed: transaction error.",
			PayloadDE: "Forschung fehlgeschlagen: Transaktionsfehler.",
		}
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 6a. Deduct each cost from inventories.
	for _, cost := range tech.Cost {
		// Get current quantity within the transaction.
		var qty int
		if err := tx.QueryRow(ctx,
			`SELECT quantity FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
			agentID, cost.Type,
		).Scan(&qty); err != nil {
			return ResearchStartResult{
				PayloadEN: fmt.Sprintf("Research failed: could not read %s inventory.", cost.Type),
				PayloadDE: fmt.Sprintf("Forschung fehlgeschlagen: %s-Inventar konnte nicht gelesen werden.", cost.Type),
			}
		}

		remaining := qty - cost.Amount
		if remaining == 0 {
			if _, err := tx.Exec(ctx,
				`DELETE FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
				agentID, cost.Type,
			); err != nil {
				return ResearchStartResult{
					PayloadEN: "Research failed: could not deduct resources.",
					PayloadDE: "Forschung fehlgeschlagen: Ressourcen konnten nicht abgezogen werden.",
				}
			}
		} else {
			if _, err := tx.Exec(ctx,
				`UPDATE inventories SET quantity = quantity - $1 WHERE agent_id = $2 AND resource_type = $3`,
				cost.Amount, agentID, cost.Type,
			); err != nil {
				return ResearchStartResult{
					PayloadEN: "Research failed: could not deduct resources.",
					PayloadDE: "Forschung fehlgeschlagen: Ressourcen konnten nicht abgezogen werden.",
				}
			}
		}
	}

	// 6b. Insert into research_queue.
	completesAt := tickNumber + int64(tech.TicksRequired)
	if _, err := tx.Exec(ctx,
		`INSERT INTO research_queue (agent_id, tech_id, started_at_tick, completes_at_tick)
		 VALUES ($1, $2, $3, $4)`,
		agentID, tech.ID, tickNumber, completesAt,
	); err != nil {
		return ResearchStartResult{
			PayloadEN: "Research failed: could not start research.",
			PayloadDE: "Forschung fehlgeschlagen: Forschung konnte nicht gestartet werden.",
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ResearchStartResult{
			PayloadEN: "Research failed: transaction commit error.",
			PayloadDE: "Forschung fehlgeschlagen: Transaktions-Commit-Fehler.",
		}
	}

	return ResearchStartResult{
		TechID:        tech.ID,
		TechName:      tech.Name,
		TicksRequired: tech.TicksRequired,
		CompletesAt:   completesAt,
		PayloadEN:     fmt.Sprintf("Research started: %s. Completes in %d ticks.", tech.Name, tech.TicksRequired),
		PayloadDE:     fmt.Sprintf("Forschung gestartet: %s. Abgeschlossen in %d Ticks.", tech.NameDE, tech.TicksRequired),
	}
}
