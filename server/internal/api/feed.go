package api

import (
	"net/http"
	"strconv"
	"time"
)

// feedEntry is a unified entry in the public agent comms feed.
type feedEntry struct {
	ID         string    `json:"id"`
	Source     string    `json:"source"`      // "agent" | "npc"
	Type       string    `json:"type"`
	AgentID    *string   `json:"agent_id,omitempty"`
	AgentName  *string   `json:"agent_name,omitempty"`
	Faction    *string   `json:"faction,omitempty"`
	FactionID  *string   `json:"faction_id,omitempty"` // NPC faction for world events
	GalaxyID   *string   `json:"galaxy_id,omitempty"`
	PayloadEN  string    `json:"payload_en"`
	PayloadDE  string    `json:"payload_de"`
	Tick       int64     `json:"tick"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleFeed serves GET /feed — public, no auth required.
// Query params:
//   ?galaxy=  — filter by galaxy_id (ships table for agent events, galaxy_id col for world events)
//   ?faction= — filter by player faction (agent events only)
//   ?type=    — filter by event type prefix (e.g. "diplomacy", "combat", "fleet_combat")
//   ?limit=   — max results, default 50, max 200
func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	galaxy  := r.URL.Query().Get("galaxy")
	faction := r.URL.Query().Get("faction")
	typeFilter := r.URL.Query().Get("type")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	ctx := r.Context()
	pool := s.registry.Pool()

	// ── Agent events ──────────────────────────────────────────────────────
	// Build dynamic query; use empty-string sentinel to mean "no filter".
	agentRows, err := pool.Query(ctx,
		`SELECT e.id, 'agent' AS source, e.type,
		        e.agent_id::text, a.name, a.faction,
		        NULL::text AS faction_id,
		        s.galaxy_id,
		        e.payload_en, e.payload_de,
		        e.tick_number, e.created_at
		 FROM events e
		 JOIN agents a ON a.id = e.agent_id
		 LEFT JOIN LATERAL (
		   SELECT galaxy_id FROM ships WHERE agent_id = e.agent_id
		   ORDER BY created_at ASC LIMIT 1
		 ) s ON true
		 WHERE e.is_public = true
		   AND ($1 = '' OR s.galaxy_id = $1)
		   AND ($2 = '' OR a.faction = $2)
		   AND ($3 = '' OR e.type LIKE $3 || '%')
		 ORDER BY e.created_at DESC
		 LIMIT $4`,
		galaxy, faction, typeFilter, limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch agent events")
		return
	}
	defer agentRows.Close()

	var entries []feedEntry
	for agentRows.Next() {
		var e feedEntry
		var agentID, agentName, agentFaction, galaxyID string
		if err := agentRows.Scan(
			&e.ID, &e.Source, &e.Type,
			&agentID, &agentName, &agentFaction,
			&e.FactionID, &galaxyID,
			&e.PayloadEN, &e.PayloadDE,
			&e.Tick, &e.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan agent event")
			return
		}
		e.AgentID   = &agentID
		e.AgentName = &agentName
		e.Faction   = &agentFaction
		e.GalaxyID  = &galaxyID
		entries = append(entries, e)
	}
	if err := agentRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "agent events rows error")
		return
	}

	// ── World events (NPC) ────────────────────────────────────────────────
	// Skip if a player-faction filter was applied (world events have no player faction).
	if faction == "" {
		worldRows, err := pool.Query(ctx,
			`SELECT id, 'npc' AS source, event_type,
			        NULL::text, NULL::text, NULL::text,
			        faction_id, galaxy_id,
			        payload_en, payload_de,
			        tick_number, created_at
			 FROM world_events
			 WHERE ($1 = '' OR galaxy_id = $1)
			   AND ($2 = '' OR event_type LIKE $2 || '%')
			 ORDER BY created_at DESC
			 LIMIT $3`,
			galaxy, typeFilter, limit,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to fetch world events")
			return
		}
		defer worldRows.Close()

		for worldRows.Next() {
			var e feedEntry
			var factionID, galaxyID string
			var agentID, agentName, agentFaction *string // always NULL for world events
			if err := worldRows.Scan(
				&e.ID, &e.Source, &e.Type,
				&agentID, &agentName, &agentFaction,
				&factionID, &galaxyID,
				&e.PayloadEN, &e.PayloadDE,
				&e.Tick, &e.CreatedAt,
			); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to scan world event")
				return
			}
			e.FactionID = &factionID
			e.GalaxyID  = &galaxyID
			entries = append(entries, e)
		}
		if err := worldRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "world events rows error")
			return
		}
	}

	// Sort merged result by created_at DESC and cap at limit.
	entries = sortAndCap(entries, limit)
	if entries == nil {
		entries = []feedEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

// sortAndCap sorts feedEntry slice by CreatedAt DESC and returns at most limit entries.
func sortAndCap(entries []feedEntry, limit int) []feedEntry {
	// Insertion sort is fine for small N (≤ 400 combined).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].CreatedAt.After(entries[j-1].CreatedAt); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	if len(entries) > limit {
		return entries[:limit]
	}
	return entries
}
