package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/hub"
)

const respawnDelay = int64(3) // ticks before auto-respawn

// checkRespawns finds every agent in 'rescue_pod' whose respawn_tick has
// arrived and gives them a fresh starter ship at their home system.
func checkRespawns(ctx context.Context, pool *pgxpool.Pool, h *hub.Hub, tickNumber int64) {
	rows, err := pool.Query(ctx,
		`SELECT a.id, a.account_id, a.name
		 FROM agents a
		 WHERE a.status = 'rescue_pod'
		   AND (a.respawn_tick IS NULL OR a.respawn_tick <= $1)`,
		tickNumber,
	)
	if err != nil {
		slog.Error("respawn: query error", "err", err)
		return
	}
	defer rows.Close()

	type pending struct {
		AgentID   string
		AccountID string
		Name      string
	}
	var agents []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.AgentID, &p.AccountID, &p.Name); err != nil {
			slog.Error("respawn: scan error", "err", err)
			continue
		}
		agents = append(agents, p)
	}
	rows.Close()

	for _, ag := range agents {
		if err := doRespawn(ctx, pool, h, ag.AgentID, ag.AccountID, ag.Name, tickNumber); err != nil {
			slog.Error("respawn: agent error", "agent", ag.AgentID, "err", err)
		}
	}
}

// doRespawn creates a new starter ship and reactivates the agent.
func doRespawn(ctx context.Context, pool *pgxpool.Pool, h *hub.Hub, agentID, accountID, agentName string, tickNumber int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Restore agent: status = active, ensure minimum 100 credits.
	if _, err := tx.Exec(ctx,
		`UPDATE agents
		 SET status = 'active', credits = GREATEST(credits, 100),
		     death_tick = NULL, respawn_tick = NULL
		 WHERE id = $1`,
		agentID,
	); err != nil {
		return fmt.Errorf("restore agent: %w", err)
	}

	// Remove destroyed ship wreck.
	if _, err := tx.Exec(ctx,
		`DELETE FROM ships WHERE agent_id = $1 AND hull_points = 0`, agentID,
	); err != nil {
		return fmt.Errorf("delete wreck: %w", err)
	}

	// Spawn fresh starter ship.
	if _, err := tx.Exec(ctx,
		`INSERT INTO ships (agent_id, name, class, hull_points, max_hull_points, galaxy_id, system_id)
		 VALUES ($1, 'Gate Runner Mk.I', 'gate_runner_mk1', 100, 100, 'milky_way', 'chulak')`,
		agentID,
	); err != nil {
		return fmt.Errorf("spawn ship: %w", err)
	}

	// Record respawn event (private).
	msgEN := fmt.Sprintf("%s recovered by rescue beacon. New ship issued at Chulak.", agentName)
	msgDE := fmt.Sprintf("%s von Rettungsbeacon geborgen. Neues Schiff auf Chulak bereitgestellt.", agentName)
	if _, err := tx.Exec(ctx,
		`INSERT INTO events (agent_id, type, payload_en, payload_de, tick_number, is_public)
		 VALUES ($1, 'respawn', $2, $3, $4, false)`,
		agentID, msgEN, msgDE, tickNumber,
	); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Notify the player via WS.
	payload, _ := json.Marshal(map[string]string{
		"payload_en": fmt.Sprintf("Rescue beacon activated. New ship issued at Chulak. Welcome back, %s.", agentName),
		"payload_de": fmt.Sprintf("Rettungsbeacon aktiviert. Neues Schiff auf Chulak bereitgestellt. Willkommen zurück, %s.", agentName),
	})
	h.SendToAgent(accountID, hub.Message{
		Type:  "event",
		Event: json.RawMessage(payload),
	})

	slog.Info("respawn: agent respawned", "agent", agentName, "id", agentID, "tick", tickNumber)
	return nil
}
