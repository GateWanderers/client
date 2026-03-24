package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/hub"
	"gatewanderers/server/internal/missions"
	"gatewanderers/server/internal/npc"
	"gatewanderers/server/internal/research"
)

// fnvHash returns a stable int64 hash of the given string using FNV-1a.
func fnvHash(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

// queuedAction holds one row from the tick_queue table.
type queuedAction struct {
	AgentID    string
	GalaxyID   string
	ActionType string
	Parameters json.RawMessage
}

// Ticker runs a periodic game-tick loop.
type Ticker struct {
	pool      *pgxpool.Pool
	hub       *hub.Hub
	npcEngine *npc.Engine
	interval  time.Duration
	stopCh    chan struct{}
	forceCh   chan struct{} // buffered(1): signals an immediate forced tick

	mu         sync.Mutex
	paused     bool
	startedAt  time.Time
	lastTickAt time.Time
	tickCount  int64
}

// New creates a Ticker with the given pool, hub, npc engine, and tick interval.
func New(pool *pgxpool.Pool, h *hub.Hub, npcEngine *npc.Engine, interval time.Duration) *Ticker {
	return &Ticker{
		pool:      pool,
		hub:       h,
		npcEngine: npcEngine,
		interval:  interval,
		stopCh:    make(chan struct{}),
		forceCh:   make(chan struct{}, 1),
	}
}

// Start begins the tick loop in a new goroutine. It stops when ctx is cancelled
// or Stop() is called.
func (t *Ticker) Start(ctx context.Context) {
	t.mu.Lock()
	t.startedAt = time.Now()
	t.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("ticker: panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.stopCh:
				return
			case <-t.forceCh:
				t.mu.Lock()
				skip := t.paused
				t.mu.Unlock()
				if skip {
					continue
				}
				if err := t.processTick(ctx); err != nil {
					slog.Error("ticker: force-tick error", "err", err)
				} else {
					t.mu.Lock()
					t.lastTickAt = time.Now()
					t.tickCount++
					t.mu.Unlock()
				}
			case <-ticker.C:
				t.mu.Lock()
				skip := t.paused
				t.mu.Unlock()
				if skip {
					continue
				}
				if err := t.processTick(ctx); err != nil {
					slog.Error("ticker: processTick error", "err", err)
				} else {
					t.mu.Lock()
					t.lastTickAt = time.Now()
					t.tickCount++
					t.mu.Unlock()
				}
			}
		}
	}()
}

// Stop signals the ticker loop to exit.
func (t *Ticker) Stop() {
	close(t.stopCh)
}

// Pause suspends automatic ticks until Resume is called.
func (t *Ticker) Pause() {
	t.mu.Lock()
	t.paused = true
	t.mu.Unlock()
}

// Resume re-enables automatic ticks.
func (t *Ticker) Resume() {
	t.mu.Lock()
	t.paused = false
	t.mu.Unlock()
}

// ForceTick schedules an immediate tick regardless of the interval.
// Non-blocking: if a force is already queued, this call is ignored.
func (t *Ticker) ForceTick() {
	select {
	case t.forceCh <- struct{}{}:
	default:
	}
}

// TickerStats holds observable state for admin monitoring.
type TickerStats struct {
	Paused     bool          `json:"paused"`
	TickCount  int64         `json:"tick_count"`
	Interval   time.Duration `json:"interval_ns"`
	StartedAt  time.Time     `json:"started_at"`
	LastTickAt time.Time     `json:"last_tick_at"`
	UptimeSec  int64         `json:"uptime_sec"`
}

// Stats returns a snapshot of the ticker's current state.
func (t *Ticker) Stats() TickerStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return TickerStats{
		Paused:     t.paused,
		TickCount:  t.tickCount,
		Interval:   t.interval,
		StartedAt:  t.startedAt,
		LastTickAt: t.lastTickAt,
		UptimeSec:  int64(time.Since(t.startedAt).Seconds()),
	}
}

// processTick performs one full tick cycle atomically.
func (t *Ticker) processTick(ctx context.Context) error {
	// 1. Increment tick_number and retrieve new value.
	var tickNumber int64
	err := t.pool.QueryRow(ctx,
		`UPDATE tick_state SET tick_number = tick_number + 1, last_tick_at = NOW()
		 WHERE id = 1 RETURNING tick_number`,
	).Scan(&tickNumber)
	if err != nil {
		return fmt.Errorf("increment tick: %w", err)
	}

	slog.Info("ticker: tick starting", "tick", tickNumber)

	// Check for completed research and pending respawns.
	t.checkResearchCompletion(ctx, tickNumber)
	checkRespawns(ctx, t.pool, t.hub, tickNumber)

	// Expire overdue missions.
	missions.ExpireOld(ctx, t.pool, tickNumber)

	// 2. Fetch all queued actions.
	rows, err := t.pool.Query(ctx,
		`SELECT agent_id, galaxy_id, action_type, parameters FROM tick_queue`,
	)
	if err != nil {
		return fmt.Errorf("fetch queue: %w", err)
	}
	var actions []queuedAction
	for rows.Next() {
		var a queuedAction
		if err := rows.Scan(&a.AgentID, &a.GalaxyID, &a.ActionType, &a.Parameters); err != nil {
			rows.Close()
			return fmt.Errorf("scan queue row: %w", err)
		}
		actions = append(actions, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("queue rows error: %w", err)
	}

	// 3. Delete all queued actions (before processing, so none are processed twice).
	if _, err := t.pool.Exec(ctx, `DELETE FROM tick_queue`); err != nil {
		return fmt.Errorf("delete queue: %w", err)
	}

	// Broadcast tick + map_update notification to all connected clients (auth and anon).
	t.hub.Broadcast(hub.Message{Type: "tick", Tick: tickNumber})
	t.hub.Broadcast(hub.Message{Type: "map_update", Tick: tickNumber})

	// 4. Resolve which agents were connected (had queued actions) so we can
	//    later notify idle agents.
	activeAgents := make(map[string]bool, len(actions))
	for _, a := range actions {
		activeAgents[a.AgentID] = true
	}

	// 5. Separate ATTACK actions for fleet-grouping; process everything else normally.
	var attackActions []queuedAction
	for _, a := range actions {
		if a.ActionType == "ATTACK" {
			attackActions = append(attackActions, a)
		} else {
			if err := t.processAction(ctx, a, tickNumber); err != nil {
				slog.Error("ticker: processAction", "agent", a.AgentID, "err", err)
			}
		}
	}

	// 5b. Resolve ATTACK actions, grouping allied agents at the same location.
	t.processAttackActions(ctx, attackActions, tickNumber)

	// 6. Provision rescue pods for agents in rescue_pod status.
	t.provisionRescuePods(ctx, tickNumber)

	// 7. Run NPC faction behaviors for this tick.
	t.npcEngine.RunTick(ctx, tickNumber, rand.New(rand.NewSource(tickNumber)))
	t.broadcastWorldEventFeed(ctx, tickNumber)

	// 8. Adjust trading post prices based on supply/demand.
	t.adjustPrices(ctx)

	// 9. Distribute system control income to controlling factions.
	t.distributeSystemIncome(ctx, tickNumber)

	// 10. Decay uncontested system defenses slightly each tick.
	t.decaySystemDefenses(ctx)

	// 11. Generate missions for active agents (up to 3 concurrent per agent).
	t.generateMissions(ctx, tickNumber)

	// Process expired auctions.
	processExpiredAuctions(ctx, t.pool, tickNumber)

	// Regenerate mining node reserves.
	t.regenMiningNodes(ctx)

	// Passive gather for agents with automated_harvesters research.
	t.passiveHarvest(ctx, tickNumber)

	// Clean up expired skill boosts.
	_, _ = t.pool.Exec(ctx, `DELETE FROM skill_boosts WHERE expires_at_tick < $1`, tickNumber)

	// Record economy snapshot for admin charts.
	t.recordStatsHistory(ctx, tickNumber)

	slog.Info("ticker: tick complete", "tick", tickNumber, "actions", len(actions))
	return nil
}

// recordStatsHistory writes a one-row economy snapshot into stats_history.
func (t *Ticker) recordStatsHistory(ctx context.Context, tickNumber int64) {
	var totalCredits int64
	var activeAgents, totalShips, playerSystems int
	_ = t.pool.QueryRow(ctx, `SELECT COALESCE(SUM(credits),0) FROM agents`).Scan(&totalCredits)
	_ = t.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agents WHERE status = 'active'`).Scan(&activeAgents)
	_ = t.pool.QueryRow(ctx, `SELECT COUNT(*) FROM ships`).Scan(&totalShips)
	_ = t.pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_control WHERE controller_type = 'player'`).Scan(&playerSystems)

	rows, err := t.pool.Query(ctx,
		`SELECT controller_faction, COUNT(*) FROM system_control
		 WHERE controller_type = 'player' GROUP BY controller_faction`)
	controlMap := map[string]int{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var faction string
			var cnt int
			if rows.Scan(&faction, &cnt) == nil {
				controlMap[faction] = cnt
			}
		}
	}
	controlJSON, _ := json.Marshal(controlMap)

	_, _ = t.pool.Exec(ctx,
		`INSERT INTO stats_history
		 (tick_number, total_credits, active_agents, total_ships, player_systems, system_control)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (tick_number) DO NOTHING`,
		tickNumber, totalCredits, activeAgents, totalShips, playerSystems, controlJSON)
}

// checkResearchCompletion checks for agents whose research has completed and finalizes it.
func (t *Ticker) checkResearchCompletion(ctx context.Context, tickNumber int64) {
	rows, err := t.pool.Query(ctx,
		`SELECT agent_id, tech_id FROM research_queue WHERE completes_at_tick <= $1`,
		tickNumber,
	)
	if err != nil {
		slog.Error("ticker: checkResearchCompletion query", "err", err)
		return
	}
	type researchEntry struct {
		AgentID string
		TechID  string
	}
	var entries []researchEntry
	for rows.Next() {
		var e researchEntry
		if err := rows.Scan(&e.AgentID, &e.TechID); err == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()

	for _, e := range entries {
		tech, ok := research.Get(e.TechID)
		if !ok {
			slog.Warn("ticker: checkResearchCompletion: unknown tech", "tech", e.TechID, "agent", e.AgentID)
			continue
		}

		// Append tech to agent's research JSONB array.
		_, err := t.pool.Exec(ctx,
			`UPDATE agents SET research = research || jsonb_build_array($2::text) WHERE id = $1`,
			e.AgentID, e.TechID,
		)
		if err != nil {
			slog.Error("ticker: checkResearchCompletion update research", "agent", e.AgentID, "err", err)
			continue
		}

		// Remove from research_queue.
		_, err = t.pool.Exec(ctx,
			`DELETE FROM research_queue WHERE agent_id = $1`,
			e.AgentID,
		)
		if err != nil {
			slog.Error("ticker: checkResearchCompletion delete queue", "agent", e.AgentID, "err", err)
			continue
		}

		// Build event payloads.
		payloadEN := fmt.Sprintf("Research complete: %s. Your capabilities have been enhanced.", tech.Name)
		payloadDE := fmt.Sprintf("Forschung abgeschlossen: %s. Deine Fähigkeiten wurden verbessert.", tech.NameDE)

		// Insert event.
		var eventID string
		err = t.pool.QueryRow(ctx,
			`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			e.AgentID, tickNumber, "research_complete", payloadEN, payloadDE, false,
		).Scan(&eventID)
		if err != nil {
			slog.Error("ticker: checkResearchCompletion insert event", "agent", e.AgentID, "err", err)
			continue
		}

		// Resolve account_id for hub delivery.
		var accountID string
		err = t.pool.QueryRow(ctx,
			`SELECT account_id FROM agents WHERE id = $1`,
			e.AgentID,
		).Scan(&accountID)
		if err != nil {
			slog.Error("ticker: checkResearchCompletion fetch account", "agent", e.AgentID, "err", err)
			continue
		}

		eventPayload, _ := json.Marshal(map[string]interface{}{
			"id":         eventID,
			"tick":       tickNumber,
			"type":       "research_complete",
			"payload_en": payloadEN,
			"payload_de": payloadDE,
		})
		t.hub.SendToAgent(accountID, hub.Message{
			Type:  "event",
			Tick:  tickNumber,
			Event: json.RawMessage(eventPayload),
		})
	}
}

// adjustPrices updates trading post prices based on supply/demand dynamics.
func (t *Ticker) adjustPrices(ctx context.Context) {
	_, err := t.pool.Exec(ctx,
		`UPDATE trading_posts SET
		  current_price = GREATEST(base_price / 2, LEAST(base_price * 3,
		    CASE
		      WHEN supply < 50  THEN ROUND(current_price * 1.05)
		      WHEN supply > 150 THEN ROUND(current_price * 0.95)
		      ELSE current_price
		    END
		  )),
		  supply = LEAST(supply + 2, 200)`,
	)
	if err != nil {
		slog.Error("ticker: adjustPrices error", "err", err)
	}
}

// provisionRescuePods assigns a new starter ship to any agent in rescue_pod status,
// resets their status to active, and notifies them via hub.
func (t *Ticker) provisionRescuePods(ctx context.Context, tickNumber int64) {
	rows, err := t.pool.Query(ctx, `SELECT id FROM agents WHERE status = 'rescue_pod'`)
	if err != nil {
		slog.Error("ticker: provisionRescuePods query", "err", err)
		return
	}
	var agentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			agentIDs = append(agentIDs, id)
		}
	}
	rows.Close()

	for _, agentID := range agentIDs {
		// Insert a new starter ship with all DB defaults.
		_, err := t.pool.Exec(ctx,
			`INSERT INTO ships (agent_id) VALUES ($1)`,
			agentID,
		)
		if err != nil {
			slog.Error("ticker: provisionRescuePods insert ship", "agent", agentID, "err", err)
			continue
		}

		// Set agent status back to active.
		_, err = t.pool.Exec(ctx,
			`UPDATE agents SET status = 'active' WHERE id = $1`,
			agentID,
		)
		if err != nil {
			slog.Error("ticker: provisionRescuePods update status", "agent", agentID, "err", err)
			continue
		}

		// Insert recovery event.
		payloadEN := "Your rescue pod has been recovered. A Gate Runner Mk.I has been provided."
		payloadDE := "Deine Rettungskapsel wurde geborgen. Ein Gate Runner Mk.I wurde bereitgestellt."
		var eventID string
		err = t.pool.QueryRow(ctx,
			`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			agentID, tickNumber, "rescue_pod_recovery", payloadEN, payloadDE, false,
		).Scan(&eventID)
		if err != nil {
			slog.Error("ticker: provisionRescuePods insert event", "agent", agentID, "err", err)
			continue
		}

		// Resolve account_id for hub delivery.
		var accountID string
		err = t.pool.QueryRow(ctx,
			`SELECT account_id FROM agents WHERE id = $1`,
			agentID,
		).Scan(&accountID)
		if err != nil {
			slog.Error("ticker: provisionRescuePods fetch account", "agent", agentID, "err", err)
			continue
		}

		eventPayload, _ := json.Marshal(map[string]interface{}{
			"id":         eventID,
			"tick":       tickNumber,
			"type":       "rescue_pod_recovery",
			"payload_en": payloadEN,
			"payload_de": payloadDE,
		})
		t.hub.SendToAgent(accountID, hub.Message{
			Type:  "event",
			Tick:  tickNumber,
			Event: json.RawMessage(eventPayload),
		})
	}
}

// processAction handles a single queued action and writes the result event.
func (t *Ticker) processAction(ctx context.Context, a queuedAction, tickNumber int64) error {
	// Resolve the agent's current system_id from their ship.
	var systemID string
	err := t.pool.QueryRow(ctx,
		`SELECT s.system_id FROM ships s WHERE s.agent_id = $1 ORDER BY s.created_at ASC LIMIT 1`,
		a.AgentID,
	).Scan(&systemID)
	if err != nil {
		return fmt.Errorf("fetch system_id: %w", err)
	}

	// Resolve accountID for hub delivery.
	var accountID string
	err = t.pool.QueryRow(ctx,
		`SELECT account_id FROM agents WHERE id = $1`,
		a.AgentID,
	).Scan(&accountID)
	if err != nil {
		return fmt.Errorf("fetch account_id: %w", err)
	}

	var payloadEN, payloadDE, eventType string

	switch a.ActionType {
	case "EXPLORE":
		// Look up the current planet name for richer text (optional — fall back to system_id).
		var planetName string
		_ = t.pool.QueryRow(ctx,
			`SELECT p.name FROM planets p
			 JOIN ships s ON s.planet_id = p.id
			 WHERE s.agent_id = $1 LIMIT 1`,
			a.AgentID,
		).Scan(&planetName)

		result := processExplore(a.AgentID, systemID, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "explore_" + result.Outcome

		// Award XP.
		if _, err := t.pool.Exec(ctx,
			`UPDATE agents SET experience = experience + 10 WHERE id = $1`,
			a.AgentID,
		); err != nil {
			return fmt.Errorf("update xp: %w", err)
		}

		// Mission tracking.
		missions.RecordExplore(ctx, t.pool, a.AgentID, tickNumber)

		// Add current planet to agent_known_planets if ship is docked at one.
		_, _ = t.pool.Exec(ctx,
			`INSERT INTO agent_known_planets (agent_id, planet_id)
			 SELECT $1, s.planet_id FROM ships s
			 WHERE s.agent_id = $1 AND s.planet_id IS NOT NULL
			 ON CONFLICT DO NOTHING`,
			a.AgentID,
		)

	case "DIAL_GATE":
		result := processDial(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		if result.Success {
			eventType = "dial_gate_success"
		} else {
			eventType = "dial_gate_fail"
		}

	case "ATTACK":
		rng := rand.New(rand.NewSource(tickNumber + fnvHash(a.AgentID)))
		result := processAttack(ctx, t.pool, a.AgentID, a.Parameters, tickNumber, rng)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "combat_" + result.Outcome

	case "GATHER":
		result := processGather(ctx, t.pool, a.AgentID, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "gather"
		if result.ResourceType != "" {
			missions.RecordGather(ctx, t.pool, a.AgentID, result.ResourceType, result.Amount, tickNumber)
		}

	case "BUY":
		result := processBuy(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "buy"

	case "SELL":
		result := processSell(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "sell"

	case "ACCEPT_TRADE":
		result := processAcceptTrade(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "accept_trade"

	case "RESEARCH":
		result := processResearch(ctx, t.pool, a.AgentID, a.Parameters, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "research_start"

	case "DIPLOMACY":
		result := processDiplomacy(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = result.EventType

	case "REPAIR":
		result := processRepair(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = result.EventType

	case "UPGRADE":
		result := processUpgrade(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = result.EventType

	case "BUY_SHIP":
		result := processBuyShip(ctx, t.pool, a.AgentID, a.Parameters)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = result.EventType

	case "DEFEND":
		result := processDefend(ctx, t.pool, a.AgentID)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "defend"

	case "MINE":
		result := processMine(ctx, t.pool, a.AgentID, a.Parameters, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "mine"
		if result.ResourceType != "" {
			missions.RecordGather(ctx, t.pool, a.AgentID, result.ResourceType, result.Amount, tickNumber)
		}

	case "SURVEY":
		result := processSurvey(ctx, t.pool, a.AgentID, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "survey"

	case "USE_SKILL":
		result := processUseSkill(ctx, t.pool, a.AgentID, a.Parameters, tickNumber)
		payloadEN = result.PayloadEN
		payloadDE = result.PayloadDE
		eventType = "use_skill"

	default:
		payloadEN = fmt.Sprintf("Unknown action type: %s", a.ActionType)
		payloadDE = fmt.Sprintf("Unbekannter Aktionstyp: %s", a.ActionType)
		eventType = "unknown"
	}

	isPublic := isPublicEventType(eventType)

	// Insert event row.
	var eventID string
	err = t.pool.QueryRow(ctx,
		`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		a.AgentID, tickNumber, eventType, payloadEN, payloadDE, isPublic,
	).Scan(&eventID)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	// Build event payload for WebSocket delivery.
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":         eventID,
		"tick":       tickNumber,
		"type":       eventType,
		"payload_en": payloadEN,
		"payload_de": payloadDE,
	})

	t.hub.SendToAgent(accountID, hub.Message{
		Type:  "event",
		Tick:  tickNumber,
		Event: json.RawMessage(eventPayload),
	})

	// Public events are also broadcast to all viewers as feed entries.
	if isPublic {
		t.broadcastFeedEntry(ctx, eventID, a.AgentID, eventType, payloadEN, payloadDE, tickNumber)
	}

	return nil
}

// locationKey groups agents by their current system+planet position.
type locationKey struct {
	SystemID string
	PlanetID string // empty string means "in system, no specific planet"
}

// processAttackActions groups ATTACK actions by location, checks alliances,
// and resolves fleet combat for allied co-located agents or single combat otherwise.
func (t *Ticker) processAttackActions(ctx context.Context, actions []queuedAction, tickNumber int64) {
	if len(actions) == 0 {
		return
	}

	// Load ship location + accountID for every attacking agent.
	type attackEntry struct {
		action    queuedAction
		accountID string
		ship      shipCombatData
	}

	entries := make([]attackEntry, 0, len(actions))
	for _, a := range actions {
		var accountID string
		if err := t.pool.QueryRow(ctx,
			`SELECT account_id FROM agents WHERE id = $1`, a.AgentID,
		).Scan(&accountID); err != nil {
			slog.Error("ticker: processAttackActions fetch accountID", "agent", a.AgentID, "err", err)
			continue
		}

		var ship shipCombatData
		if err := t.pool.QueryRow(ctx,
			`SELECT id, hull_points, max_hull_points, galaxy_id, system_id, planet_id
			 FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
			a.AgentID,
		).Scan(&ship.ID, &ship.HullPoints, &ship.MaxHullPoints,
			&ship.GalaxyID, &ship.SystemID, &ship.PlanetID); err != nil {
			slog.Error("ticker: processAttackActions fetch ship", "agent", a.AgentID, "err", err)
			continue
		}

		entries = append(entries, attackEntry{action: a, accountID: accountID, ship: ship})
	}

	// Group entries by location.
	groups := map[locationKey][]attackEntry{}
	for _, e := range entries {
		pid := ""
		if e.ship.PlanetID != nil {
			pid = *e.ship.PlanetID
		}
		key := locationKey{SystemID: e.ship.SystemID, PlanetID: pid}
		groups[key] = append(groups[key], e)
	}

	// Resolve each location group.
	for _, grp := range groups {
		rng := rand.New(rand.NewSource(tickNumber + fnvHash(grp[0].action.AgentID)))

		if len(grp) == 1 {
			t.resolveSingleAttack(ctx, grp[0].action, grp[0].accountID, grp[0].ship, tickNumber, rng)
			continue
		}

		// Check whether all agents in the group are mutually allied.
		agentIDs := make([]string, len(grp))
		for i, e := range grp {
			agentIDs[i] = e.action.AgentID
		}

		if areAllAllied(ctx, t.pool, agentIDs) {
			// Build fleet members and resolve as one engagement.
			members := make([]AgentFleetMember, len(grp))
			for i, e := range grp {
				members[i] = AgentFleetMember{
					AgentID:   e.action.AgentID,
					AccountID: e.accountID,
					Ship:      e.ship,
				}
			}
			result := processFleetAttack(ctx, t.pool, members, tickNumber, rng)
			t.broadcastFleetResult(ctx, result, tickNumber)
		} else {
			// Non-allied agents at same location fight individually.
			for _, e := range grp {
				rngLocal := rand.New(rand.NewSource(tickNumber + fnvHash(e.action.AgentID)))
				t.resolveSingleAttack(ctx, e.action, e.accountID, e.ship, tickNumber, rngLocal)
			}
		}
	}
}

// resolveSingleAttack runs processAttack and delivers the event to the agent's WebSocket.
func (t *Ticker) resolveSingleAttack(ctx context.Context, a queuedAction, accountID string, ship shipCombatData, tickNumber int64, rng *rand.Rand) {
	result := processAttack(ctx, t.pool, a.AgentID, a.Parameters, tickNumber, rng)

	if result.Outcome == "victory" {
		missions.RecordCombatVictory(ctx, t.pool, a.AgentID, tickNumber)
	}

	eventType := "combat_" + result.Outcome

	var eventID string
	err := t.pool.QueryRow(ctx,
		`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		a.AgentID, tickNumber, eventType, result.PayloadEN, result.PayloadDE, false,
	).Scan(&eventID)
	if err != nil {
		slog.Error("ticker: resolveSingleAttack insert event", "agent", a.AgentID, "err", err)
		return
	}

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"id":         eventID,
		"tick":       tickNumber,
		"type":       eventType,
		"payload_en": result.PayloadEN,
		"payload_de": result.PayloadDE,
	})
	t.hub.SendToAgent(accountID, hub.Message{
		Type:  "event",
		Tick:  tickNumber,
		Event: json.RawMessage(eventPayload),
	})
}

// publicEventTypes is the set of event types visible in the public feed.
var publicEventTypes = map[string]bool{
	"diplomacy_proposed":    true,
	"diplomacy_accepted":    true,
	"diplomacy_dissolved":   true,
	"fleet_combat_victory":  true,
	"fleet_combat_defeat":   true,
	"fleet_combat_retreat":  true,
	"combat_victory":        true,
	"combat_defeat":         true,
	"accept_trade":          true,
	"system_captured":       true,
}

// broadcastWorldEventFeed fans out any world_events created in this tick to all connected clients.
func (t *Ticker) broadcastWorldEventFeed(ctx context.Context, tickNumber int64) {
	rows, err := t.pool.Query(ctx,
		`SELECT id, faction_id, event_type, galaxy_id, payload_en, payload_de
		 FROM world_events WHERE tick_number = $1`,
		tickNumber,
	)
	if err != nil {
		slog.Error("ticker: broadcastWorldEventFeed query", "err", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, factionID, eventType, galaxyID, payloadEN, payloadDE string
		if err := rows.Scan(&id, &factionID, &eventType, &galaxyID, &payloadEN, &payloadDE); err != nil {
			continue
		}
		feedPayload, _ := json.Marshal(map[string]interface{}{
			"id":         id,
			"tick":       tickNumber,
			"type":       eventType,
			"faction_id": factionID,
			"galaxy_id":  galaxyID,
			"payload_en": payloadEN,
			"payload_de": payloadDE,
			"source":     "npc",
		})
		t.hub.Broadcast(hub.Message{Type: "feed", Event: json.RawMessage(feedPayload)})
	}
}

// isPublicEventType returns true for events that appear in the public feed.
func isPublicEventType(t string) bool { return publicEventTypes[t] }

// broadcastFeedEntry sends a feed message to all connected clients.
func (t *Ticker) broadcastFeedEntry(ctx context.Context, eventID, agentID, eventType, payloadEN, payloadDE string, tickNumber int64) {
	// Enrich with agent name + faction for the feed.
	var agentName, faction string
	_ = t.pool.QueryRow(ctx,
		`SELECT name, faction FROM agents WHERE id = $1`, agentID,
	).Scan(&agentName, &faction)

	feedPayload, _ := json.Marshal(map[string]interface{}{
		"id":          eventID,
		"tick":        tickNumber,
		"type":        eventType,
		"agent_id":    agentID,
		"agent_name":  agentName,
		"faction":     faction,
		"payload_en":  payloadEN,
		"payload_de":  payloadDE,
		"source":      "agent",
	})
	t.hub.Broadcast(hub.Message{Type: "feed", Event: json.RawMessage(feedPayload)})
}

// broadcastFleetResult delivers individual combat events to each fleet member.
func (t *Ticker) broadcastFleetResult(ctx context.Context, result FleetCombatResult, tickNumber int64) {
	eventType := "fleet_combat_" + result.Outcome

	for _, m := range result.Members {
		var eventID string
		err := t.pool.QueryRow(ctx,
			`INSERT INTO events (agent_id, tick_number, type, payload_en, payload_de, is_public)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			m.AgentID, tickNumber, eventType, m.PayloadEN, m.PayloadDE, true,
		).Scan(&eventID)
		if err != nil {
			slog.Error("ticker: broadcastFleetResult insert event", "agent", m.AgentID, "err", err)
			continue
		}

		eventPayload, _ := json.Marshal(map[string]interface{}{
			"id":         eventID,
			"tick":       tickNumber,
			"type":       eventType,
			"payload_en": m.PayloadEN,
			"payload_de": m.PayloadDE,
		})
		t.hub.SendToAgent(m.AccountID, hub.Message{
			Type:  "event",
			Tick:  tickNumber,
			Event: json.RawMessage(eventPayload),
		})

		// Fleet combat is always public — broadcast to the feed once (first member).
		if err == nil && m.AgentID == result.Members[0].AgentID {
			t.broadcastFeedEntry(ctx, eventID, m.AgentID, eventType, m.PayloadEN, m.PayloadDE, tickNumber)
		}
	}
}

// distributeSystemIncome pays income_per_tick credits to all agents whose faction
// controls a player-held system. Each agent in the controlling faction receives
// an equal share of the system's income.
func (t *Ticker) distributeSystemIncome(ctx context.Context, tickNumber int64) {
	// Fetch all player-controlled systems.
	rows, err := t.pool.Query(ctx,
		`SELECT system_id, galaxy_id, controller_faction, income_per_tick
		 FROM system_control
		 WHERE controller_type = 'player'`,
	)
	if err != nil {
		slog.Error("ticker: distributeSystemIncome query", "err", err)
		return
	}
	type sysRow struct {
		SystemID  string
		GalaxyID  string
		Faction   string
		IncomePer int
	}
	var systems []sysRow
	for rows.Next() {
		var s sysRow
		if err := rows.Scan(&s.SystemID, &s.GalaxyID, &s.Faction, &s.IncomePer); err == nil {
			systems = append(systems, s)
		}
	}
	rows.Close()

	for _, sys := range systems {
		// Count active agents in this faction.
		var count int
		if err := t.pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM agents WHERE faction = $1 AND status = 'active'`,
			sys.Faction,
		).Scan(&count); err != nil || count == 0 {
			continue
		}

		share := sys.IncomePer / count
		if share < 1 {
			share = 1
		}

		// Pay each agent in the faction.
		if _, err := t.pool.Exec(ctx,
			`UPDATE agents SET credits = credits + $1
			 WHERE faction = $2 AND status = 'active'`,
			share, sys.Faction,
		); err != nil {
			slog.Error("ticker: distributeSystemIncome pay", "faction", sys.Faction, "system", sys.SystemID, "err", err)
		}
	}
}

// decaySystemDefenses reduces the defense_strength of player-held systems by 1
// per tick (simulating maintenance requirements). Defense never drops below 0.
func (t *Ticker) decaySystemDefenses(ctx context.Context) {
	_, err := t.pool.Exec(ctx,
		`UPDATE system_control
		 SET defense_strength = GREATEST(0, defense_strength - 1),
		     updated_at = NOW()
		 WHERE controller_type = 'player'`,
	)
	if err != nil {
		slog.Error("ticker: decaySystemDefenses", "err", err)
	}
}

// regenMiningNodes restores reserves on all mining nodes by their regen_per_tick amount.
func (t *Ticker) regenMiningNodes(ctx context.Context) {
	_, err := t.pool.Exec(ctx,
		`UPDATE mining_nodes
		 SET current_reserves = LEAST(max_reserves, current_reserves + regen_per_tick)`,
	)
	if err != nil {
		slog.Error("ticker: regenMiningNodes", "err", err)
	}
}

// passiveHarvest gives a small automatic resource yield to agents with automated_harvesters research.
func (t *Ticker) passiveHarvest(ctx context.Context, tickNumber int64) {
	// Find all active agents with automated_harvesters in their research.
	rows, err := t.pool.Query(ctx,
		`SELECT a.id, s.system_id
		 FROM agents a
		 JOIN ships s ON s.agent_id = a.id
		 WHERE a.status = 'active'
		   AND a.research @> '["automated_harvesters"]'::jsonb`,
	)
	if err != nil {
		slog.Error("ticker: passiveHarvest query", "err", err)
		return
	}
	type entry struct{ AgentID, SystemID string }
	var entries []entry
	for rows.Next() {
		var e entry
		if rows.Scan(&e.AgentID, &e.SystemID) == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()

	rng := rand.New(rand.NewSource(tickNumber + 77777))
	for _, e := range entries {
		// Find a node in the agent's current system.
		var resourceType string
		var reserves int
		if err := t.pool.QueryRow(ctx,
			`SELECT resource_type, current_reserves FROM mining_nodes
			 WHERE system_id = $1 AND current_reserves > 0
			 ORDER BY current_reserves DESC LIMIT 1`,
			e.SystemID,
		).Scan(&resourceType, &reserves); err != nil {
			continue // no resources here
		}
		amount := 3 + rng.Intn(6) // 3–8 units
		if amount > reserves {
			amount = reserves
		}
		_, _ = t.pool.Exec(ctx,
			`INSERT INTO inventories (agent_id, resource_type, quantity) VALUES ($1, $2, $3)
			 ON CONFLICT (agent_id, resource_type) DO UPDATE SET quantity = inventories.quantity + EXCLUDED.quantity`,
			e.AgentID, resourceType, amount,
		)
		_, _ = t.pool.Exec(ctx,
			`UPDATE mining_nodes SET current_reserves = GREATEST(0, current_reserves - $1) WHERE system_id = $2 AND resource_type = $3`,
			amount, e.SystemID, resourceType,
		)
	}
}

// generateMissions offers new missions to all active agents who have fewer than 3 active.
func (t *Ticker) generateMissions(ctx context.Context, tickNumber int64) {
	rows, err := t.pool.Query(ctx, `SELECT id FROM agents WHERE status = 'active'`)
	if err != nil {
		slog.Error("ticker: generateMissions query", "err", err)
		return
	}
	var agentIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			agentIDs = append(agentIDs, id)
		}
	}
	rows.Close()

	rng := rand.New(rand.NewSource(tickNumber))
	for _, id := range agentIDs {
		missions.GenerateForAgent(ctx, t.pool, id, tickNumber, rng)
	}
}
