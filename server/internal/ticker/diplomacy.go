package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DiplomacyResult holds the outcome of a DIPLOMACY action.
type DiplomacyResult struct {
	PayloadEN string
	PayloadDE string
	EventType string
}

// diplomacyParams are the JSON parameters for a DIPLOMACY action.
type diplomacyParams struct {
	Action        string `json:"action"`
	TargetAgentID string `json:"target_agent_id"`
}

// processDiplomacy dispatches PROPOSE_ALLIANCE, ACCEPT_ALLIANCE, or DISSOLVE_ALLIANCE.
func processDiplomacy(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) DiplomacyResult {
	var p diplomacyParams
	if err := json.Unmarshal(params, &p); err != nil || p.TargetAgentID == "" {
		return DiplomacyResult{
			PayloadEN: "Invalid diplomacy parameters.",
			PayloadDE: "Ungültige Diplomatie-Parameter.",
			EventType: "diplomacy_error",
		}
	}
	if p.TargetAgentID == agentID {
		return DiplomacyResult{
			PayloadEN: "You cannot form an alliance with yourself.",
			PayloadDE: "Du kannst keine Allianz mit dir selbst eingehen.",
			EventType: "diplomacy_error",
		}
	}

	switch p.Action {
	case "PROPOSE_ALLIANCE":
		return proposeAlliance(ctx, pool, agentID, p.TargetAgentID)
	case "ACCEPT_ALLIANCE":
		return acceptAlliance(ctx, pool, agentID, p.TargetAgentID)
	case "DISSOLVE_ALLIANCE":
		return dissolveAlliance(ctx, pool, agentID, p.TargetAgentID)
	default:
		return DiplomacyResult{
			PayloadEN: fmt.Sprintf("Unknown diplomacy action: %s", p.Action),
			PayloadDE: fmt.Sprintf("Unbekannte Diplomatie-Aktion: %s", p.Action),
			EventType: "diplomacy_error",
		}
	}
}

// agentName fetches an agent's display name, falling back to the raw ID.
func agentName(ctx context.Context, pool *pgxpool.Pool, agentID string) string {
	var name string
	_ = pool.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, agentID).Scan(&name)
	if name == "" {
		return agentID
	}
	return name
}

// proposeAlliance creates or refreshes a pending alliance offer from proposerID to targetID.
func proposeAlliance(ctx context.Context, pool *pgxpool.Pool, proposerID, targetID string) DiplomacyResult {
	var targetName string
	if err := pool.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, targetID).Scan(&targetName); err != nil {
		return DiplomacyResult{
			PayloadEN: "Target agent not found.",
			PayloadDE: "Zielagent nicht gefunden.",
			EventType: "diplomacy_error",
		}
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO alliances (proposer_id, target_id, status)
		 VALUES ($1, $2, 'pending')
		 ON CONFLICT (proposer_id, target_id)
		 DO UPDATE SET status = 'pending', created_at = NOW()`,
		proposerID, targetID,
	)
	if err != nil {
		slog.Error("ticker: proposeAlliance insert", "err", err)
		return DiplomacyResult{
			PayloadEN: "Failed to send alliance proposal.",
			PayloadDE: "Allianzvorschlag konnte nicht gesendet werden.",
			EventType: "diplomacy_error",
		}
	}

	return DiplomacyResult{
		PayloadEN: fmt.Sprintf("Alliance proposal sent to %s. They can accept it next tick with DIPLOMACY ACCEPT_ALLIANCE.", targetName),
		PayloadDE: fmt.Sprintf("Allianzvorschlag an %s gesendet. Sie können ihn beim nächsten Tick mit DIPLOMACY ACCEPT_ALLIANCE annehmen.", targetName),
		EventType: "diplomacy_proposed",
	}
}

// acceptAlliance promotes a pending offer (proposerID → agentID) to active.
func acceptAlliance(ctx context.Context, pool *pgxpool.Pool, agentID, proposerID string) DiplomacyResult {
	var allianceID string
	err := pool.QueryRow(ctx,
		`SELECT id FROM alliances
		 WHERE proposer_id = $1 AND target_id = $2 AND status = 'pending'`,
		proposerID, agentID,
	).Scan(&allianceID)
	if err != nil {
		return DiplomacyResult{
			PayloadEN: "No pending alliance offer found from that agent.",
			PayloadDE: "Kein ausstehender Allianzvorschlag von diesem Agenten gefunden.",
			EventType: "diplomacy_error",
		}
	}

	if _, err = pool.Exec(ctx,
		`UPDATE alliances SET status = 'active' WHERE id = $1`, allianceID,
	); err != nil {
		slog.Error("ticker: acceptAlliance update", "err", err)
		return DiplomacyResult{
			PayloadEN: "Failed to accept alliance.",
			PayloadDE: "Allianz konnte nicht akzeptiert werden.",
			EventType: "diplomacy_error",
		}
	}

	partnerName := agentName(ctx, pool, proposerID)
	return DiplomacyResult{
		PayloadEN: fmt.Sprintf("Alliance with %s is now active. You will fight as a combined fleet when co-located.", partnerName),
		PayloadDE: fmt.Sprintf("Allianz mit %s ist jetzt aktiv. Ihr kämpft als kombinierte Flotte, wenn ihr am selben Ort seid.", partnerName),
		EventType: "diplomacy_accepted",
	}
}

// dissolveAlliance removes the alliance record in both directions.
func dissolveAlliance(ctx context.Context, pool *pgxpool.Pool, agentID, partnerID string) DiplomacyResult {
	ct, err := pool.Exec(ctx,
		`DELETE FROM alliances
		 WHERE (proposer_id = $1 AND target_id = $2)
		    OR (proposer_id = $2 AND target_id = $1)`,
		agentID, partnerID,
	)
	if err != nil {
		slog.Error("ticker: dissolveAlliance delete", "err", err)
		return DiplomacyResult{
			PayloadEN: "Failed to dissolve alliance.",
			PayloadDE: "Allianz konnte nicht aufgelöst werden.",
			EventType: "diplomacy_error",
		}
	}
	if ct.RowsAffected() == 0 {
		return DiplomacyResult{
			PayloadEN: "No alliance with that agent to dissolve.",
			PayloadDE: "Keine Allianz mit diesem Agenten zum Auflösen.",
			EventType: "diplomacy_error",
		}
	}

	partnerName := agentName(ctx, pool, partnerID)
	return DiplomacyResult{
		PayloadEN: fmt.Sprintf("Alliance with %s has been dissolved.", partnerName),
		PayloadDE: fmt.Sprintf("Allianz mit %s wurde aufgelöst.", partnerName),
		EventType: "diplomacy_dissolved",
	}
}

// areAllAllied returns true if every pair in agentIDs has an active alliance.
func areAllAllied(ctx context.Context, pool *pgxpool.Pool, agentIDs []string) bool {
	for i := 0; i < len(agentIDs); i++ {
		for j := i + 1; j < len(agentIDs); j++ {
			var count int
			err := pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM alliances
				 WHERE status = 'active'
				   AND ((proposer_id = $1 AND target_id = $2)
				     OR (proposer_id = $2 AND target_id = $1))`,
				agentIDs[i], agentIDs[j],
			).Scan(&count)
			if err != nil || count == 0 {
				return false
			}
		}
	}
	return true
}
