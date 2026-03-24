package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/research"
)

const surveyBaseDuration = int64(15) // ticks before a survey expires

type surveyResult struct {
	SystemID  string
	NodeCount int
	PayloadEN string
	PayloadDE string
}

// processSurvey handles the SURVEY action — reveals mining node data for the current system.
func processSurvey(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64) surveyResult {
	// Get current system.
	var systemID string
	err := pool.QueryRow(ctx,
		`SELECT system_id FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&systemID)
	if err != nil {
		return surveyResult{PayloadEN: "Survey failed: ship not found.", PayloadDE: "Survey fehlgeschlagen: Schiff nicht gefunden."}
	}

	// Load research for survey duration multiplier.
	var researchRaw []byte
	_ = pool.QueryRow(ctx, `SELECT research FROM agents WHERE id = $1`, agentID).Scan(&researchRaw)
	var completedResearch []string
	_ = json.Unmarshal(researchRaw, &completedResearch)

	multiplier := research.GeologicalSurveyBonus(completedResearch)
	duration := int64(float64(surveyBaseDuration) * multiplier)
	expiresAt := tickNumber + duration

	// Count mining nodes in this system.
	var nodeCount int
	_ = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM mining_nodes WHERE system_id = $1`,
		systemID,
	).Scan(&nodeCount)

	if nodeCount == 0 {
		return surveyResult{
			SystemID:  systemID,
			NodeCount: 0,
			PayloadEN: fmt.Sprintf("Survey of %s complete: no mineral deposits detected.", systemID),
			PayloadDE: fmt.Sprintf("Erkundung von %s abgeschlossen: Keine Mineralvorkommen gefunden.", systemID),
		}
	}

	// Upsert survey record.
	_, err = pool.Exec(ctx,
		`INSERT INTO surveys (agent_id, system_id, surveyed_at_tick, expires_at_tick)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (agent_id, system_id) DO UPDATE
		   SET surveyed_at_tick = EXCLUDED.surveyed_at_tick,
		       expires_at_tick  = EXCLUDED.expires_at_tick`,
		agentID, systemID, tickNumber, expiresAt,
	)
	if err != nil {
		slog.Error("processSurvey: upsert survey", "agent", agentID, "err", err)
		return surveyResult{PayloadEN: "Survey failed: database error.", PayloadDE: "Survey fehlgeschlagen: Datenbankfehler."}
	}

	// Award XP.
	_, _ = pool.Exec(ctx, `UPDATE agents SET experience = experience + 8 WHERE id = $1`, agentID)

	return surveyResult{
		SystemID:  systemID,
		NodeCount: nodeCount,
		PayloadEN: fmt.Sprintf("Survey of %s complete: %d mineral deposit(s) catalogued. Data valid for %d ticks.",
			systemID, nodeCount, duration),
		PayloadDE: fmt.Sprintf("Erkundung von %s abgeschlossen: %d Mineralvorkommen katalogisiert. Daten gültig für %d Ticks.",
			systemID, nodeCount, duration),
	}
}
