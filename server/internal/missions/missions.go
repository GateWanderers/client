// Package missions handles short-term generated quests for agents.
package missions

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
)

// missionTemplate defines one possible mission shape.
type missionTemplate struct {
	Type       string
	TitleEN    string
	TitleDE    string
	DescEN     string
	DescDE     string
	Resource   string // empty for non-resource missions
	Quantity   int
	Credits    int
	XP         int
	Duration   int64 // duration in ticks
}

var templates = []missionTemplate{
	{
		Type:     "explore",
		TitleEN:  "Scout the Sector",
		TitleDE:  "Sektor erkunden",
		DescEN:   "Perform 3 EXPLORE actions to chart nearby space.",
		DescDE:   "Führe 3 EXPLORE-Aktionen durch, um den Nahbereich zu kartografieren.",
		Quantity: 3,
		Credits:  150,
		XP:       30,
		Duration: 10,
	},
	{
		Type:     "gather",
		TitleEN:  "Naquadah Extraction",
		TitleDE:  "Naquadah-Extraktion",
		DescEN:   "Gather 50 units of naquadah for the SGC.",
		DescDE:   "Sammle 50 Einheiten Naquadah für das SGC.",
		Resource: "naquadah",
		Quantity: 50,
		Credits:  200,
		XP:       40,
		Duration: 8,
	},
	{
		Type:     "gather",
		TitleEN:  "Trinium Supply Run",
		TitleDE:  "Trinium-Versorgung",
		DescEN:   "Gather 30 units of trinium for ship construction.",
		DescDE:   "Sammle 30 Einheiten Trinium für den Schiffsbau.",
		Resource: "trinium",
		Quantity: 30,
		Credits:  180,
		XP:       35,
		Duration: 8,
	},
	{
		Type:     "attack",
		TitleEN:  "Hostile Elimination",
		TitleDE:  "Bedrohung neutralisieren",
		DescEN:   "Defeat 2 NPC encounters to secure the system.",
		DescDE:   "Besiege 2 NPC-Begegnungen, um das System zu sichern.",
		Quantity: 2,
		Credits:  250,
		XP:       60,
		Duration: 6,
	},
	{
		Type:     "gather",
		TitleEN:  "Neutronium Retrieval",
		TitleDE:  "Neutronium-Bergung",
		DescEN:   "Gather 20 units of neutronium from deep space.",
		DescDE:   "Sammle 20 Einheiten Neutronium aus dem Weltraum.",
		Resource: "neutronium",
		Quantity: 20,
		Credits:  300,
		XP:       50,
		Duration: 10,
	},
	{
		Type:     "explore",
		TitleEN:  "Deep Space Reconnaissance",
		TitleDE:  "Tiefraum-Aufklärung",
		DescEN:   "Perform 5 EXPLORE actions to map uncharted territories.",
		DescDE:   "Führe 5 EXPLORE-Aktionen durch, um unerforschte Gebiete zu kartieren.",
		Quantity: 5,
		Credits:  300,
		XP:       70,
		Duration: 12,
	},
	{
		Type:     "attack",
		TitleEN:  "Purge the Invaders",
		TitleDE:  "Eindringlinge vertreiben",
		DescEN:   "Defeat 3 NPC encounters to push back the invasion.",
		DescDE:   "Besiege 3 NPC-Begegnungen, um die Invasion zurückzuschlagen.",
		Quantity: 3,
		Credits:  400,
		XP:       90,
		Duration: 8,
	},
}

// GenerateForAgent creates one new mission for the agent if they have fewer than 3 active.
func GenerateForAgent(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64, rng *rand.Rand) {
	// Count active missions.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM missions WHERE agent_id = $1 AND status = 'active'`,
		agentID,
	).Scan(&count); err != nil {
		slog.Error("missions: count active", "agent", agentID, "err", err)
		return
	}
	if count >= 3 {
		return
	}

	t := templates[rng.Intn(len(templates))]

	_, err := pool.Exec(ctx,
		`INSERT INTO missions
		 (agent_id, type, title_en, title_de, desc_en, desc_de,
		  target_resource, target_quantity, reward_credits, reward_xp, expires_at_tick)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		agentID, t.Type, t.TitleEN, t.TitleDE, t.DescEN, t.DescDE,
		nullStr(t.Resource), t.Quantity, t.Credits, t.XP, tickNumber+t.Duration,
	)
	if err != nil {
		slog.Error("missions: insert", "agent", agentID, "err", err)
	}
}

// ExpireOld marks overdue active missions as expired.
func ExpireOld(ctx context.Context, pool *pgxpool.Pool, tickNumber int64) {
	if _, err := pool.Exec(ctx,
		`UPDATE missions SET status = 'expired'
		 WHERE status = 'active' AND expires_at_tick < $1`,
		tickNumber,
	); err != nil {
		slog.Error("missions: expire", "err", err)
	}
}

// RecordExplore increments progress on any active explore missions for the agent.
func RecordExplore(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64) {
	recordProgress(ctx, pool, agentID, "explore", "", 1, tickNumber)
}

// RecordGather increments progress on gather missions matching the resource type.
func RecordGather(ctx context.Context, pool *pgxpool.Pool, agentID, resource string, amount int, tickNumber int64) {
	recordProgress(ctx, pool, agentID, "gather", resource, amount, tickNumber)
}

// RecordCombatVictory increments progress on attack missions.
func RecordCombatVictory(ctx context.Context, pool *pgxpool.Pool, agentID string, tickNumber int64) {
	recordProgress(ctx, pool, agentID, "attack", "", 1, tickNumber)
}

// recordProgress is the shared helper that advances mission progress and completes if done.
func recordProgress(ctx context.Context, pool *pgxpool.Pool, agentID, missionType, resource string, amount int, tickNumber int64) {
	// Fetch matching active missions.
	q := `SELECT id, progress, target_quantity, reward_credits, reward_xp
	      FROM missions
	      WHERE agent_id = $1 AND type = $2 AND status = 'active'`
	args := []interface{}{agentID, missionType}
	if resource != "" {
		q += ` AND target_resource = $3`
		args = append(args, resource)
	}

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		slog.Error("missions: recordProgress query", "agent", agentID, "type", missionType, "err", err)
		return
	}

	type mRow struct {
		ID              string
		Progress        int
		TargetQuantity  int
		RewardCredits   int
		RewardXP        int
	}
	var missions []mRow
	for rows.Next() {
		var m mRow
		if err := rows.Scan(&m.ID, &m.Progress, &m.TargetQuantity, &m.RewardCredits, &m.RewardXP); err == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()

	for _, m := range missions {
		newProgress := m.Progress + amount
		if newProgress >= m.TargetQuantity {
			// Complete mission + pay reward.
			if _, err := pool.Exec(ctx,
				`UPDATE missions SET status='completed', progress=$1, completed_at_tick=$2 WHERE id=$3`,
				newProgress, tickNumber, m.ID,
			); err != nil {
				slog.Error("missions: complete", "mission", m.ID, "err", err)
				continue
			}
			if _, err := pool.Exec(ctx,
				`UPDATE agents SET credits = credits + $1, experience = experience + $2 WHERE id = $3`,
				m.RewardCredits, m.RewardXP, agentID,
			); err != nil {
				slog.Error("missions: pay reward", "agent", agentID, "mission", m.ID, "err", err)
			}
			insertCompletionEvent(ctx, pool, agentID, m.ID, m.RewardCredits, m.RewardXP, tickNumber)
		} else {
			if _, err := pool.Exec(ctx,
				`UPDATE missions SET progress=$1 WHERE id=$2`,
				newProgress, m.ID,
			); err != nil {
				slog.Error("missions: update progress", "mission", m.ID, "err", err)
			}
		}
	}
}

// insertCompletionEvent creates an event row for a completed mission.
func insertCompletionEvent(ctx context.Context, pool *pgxpool.Pool, agentID, missionID string, credits, xp int, tickNumber int64) {
	payloadEN := fmt.Sprintf("Mission complete! Reward: %d credits, %d XP.", credits, xp)
	payloadDE := fmt.Sprintf("Mission abgeschlossen! Belohnung: %d Credits, %d EP.", credits, xp)
	_, err := pool.Exec(ctx,
		`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
		 VALUES ($1, $2, 'mission_complete', $3, $4, false)`,
		agentID, tickNumber, payloadEN, payloadDE,
	)
	if err != nil {
		slog.Error("missions: insertCompletionEvent", "agent", agentID, "mission", missionID, "err", err)
	}
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
