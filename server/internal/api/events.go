package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// worldEvent represents a single world event entry.
type worldEvent struct {
	FactionID  string    `json:"faction_id"`
	EventType  string    `json:"event_type"`
	GalaxyID   string    `json:"galaxy_id"`
	SystemID   *string   `json:"system_id"`
	PayloadEN  string    `json:"payload_en"`
	PayloadDE  string    `json:"payload_de"`
	TickNumber int64     `json:"tick_number"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleWorldEvents returns recent world events.
// GET /events (public, no auth)
// Optional query params: ?galaxy=milky_way, ?faction=wraith, ?type=culling
func (s *Server) handleWorldEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pool := s.registry.Pool()

	galaxy := r.URL.Query().Get("galaxy")
	faction := r.URL.Query().Get("faction")
	eventType := r.URL.Query().Get("type")

	query := `SELECT faction_id, event_type, galaxy_id, system_id, payload_en, payload_de, tick_number, created_at
	          FROM world_events
	          WHERE ($1::text IS NULL OR galaxy_id = $1)
	            AND ($2::text IS NULL OR faction_id = $2)
	            AND ($3::text IS NULL OR event_type = $3)
	          ORDER BY created_at DESC LIMIT 50`

	var galaxyParam, factionParam, typeParam interface{}
	if galaxy != "" {
		galaxyParam = galaxy
	}
	if faction != "" {
		factionParam = faction
	}
	if eventType != "" {
		typeParam = eventType
	}

	rows, err := pool.Query(ctx, query, galaxyParam, factionParam, typeParam)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch world events")
		return
	}
	defer rows.Close()

	events := []worldEvent{}
	for rows.Next() {
		var e worldEvent
		if err := rows.Scan(
			&e.FactionID,
			&e.EventType,
			&e.GalaxyID,
			&e.SystemID,
			&e.PayloadEN,
			&e.PayloadDE,
			&e.TickNumber,
			&e.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan world event row")
			return
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "world events rows error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}

// reputationEntry represents an agent's reputation with one NPC faction.
type reputationEntry struct {
	FactionID   string `json:"faction_id"`
	FactionName string `json:"faction_name"`
	Score       int    `json:"score"`
}

// handleAgentReputation returns the authenticated agent's NPC faction reputation.
// GET /agent/reputation (auth required)
func (s *Server) handleAgentReputation(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()
	pool := s.registry.Pool()

	// Fetch the agent's reputation JSONB.
	var agentID string
	var reputationRaw json.RawMessage
	err := pool.QueryRow(ctx,
		`SELECT id, reputation FROM agents WHERE account_id = $1`,
		accountID,
	).Scan(&agentID, &reputationRaw)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Parse the reputation JSON into a map.
	var reputationMap map[string]int
	if len(reputationRaw) > 0 {
		if err := json.Unmarshal(reputationRaw, &reputationMap); err != nil {
			reputationMap = map[string]int{}
		}
	} else {
		reputationMap = map[string]int{}
	}

	// Fetch all NPC factions for name enrichment.
	rows, err := pool.Query(ctx, `SELECT id, name FROM npc_factions ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch npc factions")
		return
	}
	defer rows.Close()

	factionNames := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			factionNames[id] = name
		}
	}
	rows.Close()

	// Build enriched reputation list.
	reputation := []reputationEntry{}
	for factionID, factionName := range factionNames {
		score := reputationMap[factionID] // defaults to 0 if not present
		reputation = append(reputation, reputationEntry{
			FactionID:   factionID,
			FactionName: factionName,
			Score:       score,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"reputation": reputation})
}
