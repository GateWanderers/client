package ticker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefendResult holds the outcome of a DEFEND action.
type DefendResult struct {
	PayloadEN string
	PayloadDE string
}

// processDefend executes the DEFEND action: reinforces the defense of the system
// the agent currently occupies. Requires the agent's faction to control the system.
// Awards +5 XP on success.
func processDefend(ctx context.Context, pool *pgxpool.Pool, agentID string) DefendResult {
	// Fetch agent faction + ship location.
	var faction, systemID, galaxyID string
	err := pool.QueryRow(ctx,
		`SELECT a.faction, s.system_id, s.galaxy_id
		 FROM agents a
		 JOIN ships s ON s.agent_id = a.id
		 WHERE a.id = $1
		 ORDER BY s.created_at ASC LIMIT 1`,
		agentID,
	).Scan(&faction, &systemID, &galaxyID)
	if err != nil {
		slog.Error("processDefend: fetch agent/ship", "agent", agentID, "err", err)
		return DefendResult{
			PayloadEN: "System defense failed: could not locate your ship.",
			PayloadDE: "Systemverteidigung fehlgeschlagen: Schiff konnte nicht gefunden werden.",
		}
	}

	// Check that agent's faction controls this system.
	var controllerFaction, controllerType string
	var currentDefense int
	err = pool.QueryRow(ctx,
		`SELECT controller_faction, controller_type, defense_strength
		 FROM system_control
		 WHERE system_id = $1 AND galaxy_id = $2`,
		systemID, galaxyID,
	).Scan(&controllerFaction, &controllerType, &currentDefense)
	if err != nil {
		return DefendResult{
			PayloadEN: fmt.Sprintf("System %s has no control record — cannot defend.", systemID),
			PayloadDE: fmt.Sprintf("System %s hat keinen Kontrolleintrag — Verteidigung nicht möglich.", systemID),
		}
	}

	if controllerType != "player" || controllerFaction != faction {
		return DefendResult{
			PayloadEN: fmt.Sprintf("Your faction does not control %s. You can only defend systems your faction owns.", systemID),
			PayloadDE: fmt.Sprintf("Deine Fraktion kontrolliert %s nicht. Du kannst nur Systeme verteidigen, die deine Fraktion kontrolliert.", systemID),
		}
	}

	// Increase defense strength by 15, cap at 100.
	const repairAmount = 15
	const maxDefense = 100
	newDefense := currentDefense + repairAmount
	if newDefense > maxDefense {
		newDefense = maxDefense
	}

	_, err = pool.Exec(ctx,
		`UPDATE system_control SET defense_strength = $1, updated_at = NOW()
		 WHERE system_id = $2 AND galaxy_id = $3`,
		newDefense, systemID, galaxyID,
	)
	if err != nil {
		slog.Error("processDefend: update defense", "agent", agentID, "system", systemID, "err", err)
		return DefendResult{
			PayloadEN: "System defense failed: database error.",
			PayloadDE: "Systemverteidigung fehlgeschlagen: Datenbankfehler.",
		}
	}

	// Award +5 XP.
	if _, err := pool.Exec(ctx,
		`UPDATE agents SET experience = experience + 5 WHERE id = $1`,
		agentID,
	); err != nil {
		slog.Error("processDefend: award xp", "agent", agentID, "err", err)
	}

	return DefendResult{
		PayloadEN: fmt.Sprintf("Defensive perimeter reinforced in %s. Defense strength: %d → %d.", systemID, currentDefense, newDefense),
		PayloadDE: fmt.Sprintf("Verteidigungsperimeter in %s verstärkt. Verteidigungsstärke: %d → %d.", systemID, currentDefense, newDefense),
	}
}
