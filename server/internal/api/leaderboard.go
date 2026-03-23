package api

import (
	"net/http"
	"time"
)

// handleLeaderboard serves GET /leaderboard — public, no auth required.
// Returns top agents by credits, by XP, and faction power aggregate.
func (s *Server) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pool := s.registry.Pool()

	type agentRow struct {
		AgentID    string    `json:"agent_id"`
		Name       string    `json:"name"`
		Faction    string    `json:"faction"`
		Credits    int64     `json:"credits"`
		Experience int64     `json:"experience"`
		Ships      int64     `json:"ships"`
		CreatedAt  time.Time `json:"created_at"`
	}

	// Top 20 by credits.
	creditRows, err := pool.Query(ctx,
		`SELECT a.id, a.name, a.faction, a.credits, a.experience,
		        (SELECT COUNT(*) FROM ships s WHERE s.agent_id = a.id AND s.hull_points > 0) AS ships,
		        a.created_at
		 FROM agents a
		 WHERE a.status != 'rescue_pod' AND (a.banned_at IS NULL)
		 ORDER BY a.credits DESC
		 LIMIT 20`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leaderboard query failed")
		return
	}
	defer creditRows.Close()

	var topCredits []agentRow
	for creditRows.Next() {
		var row agentRow
		if err := creditRows.Scan(&row.AgentID, &row.Name, &row.Faction,
			&row.Credits, &row.Experience, &row.Ships, &row.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		topCredits = append(topCredits, row)
	}
	if topCredits == nil {
		topCredits = []agentRow{}
	}

	// Top 20 by XP.
	xpRows, err := pool.Query(ctx,
		`SELECT a.id, a.name, a.faction, a.credits, a.experience,
		        (SELECT COUNT(*) FROM ships s WHERE s.agent_id = a.id AND s.hull_points > 0) AS ships,
		        a.created_at
		 FROM agents a
		 WHERE a.status != 'rescue_pod' AND (a.banned_at IS NULL)
		 ORDER BY a.experience DESC
		 LIMIT 20`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "xp leaderboard query failed")
		return
	}
	defer xpRows.Close()

	var topXP []agentRow
	for xpRows.Next() {
		var row agentRow
		if err := xpRows.Scan(&row.AgentID, &row.Name, &row.Faction,
			&row.Credits, &row.Experience, &row.Ships, &row.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		topXP = append(topXP, row)
	}
	if topXP == nil {
		topXP = []agentRow{}
	}

	// Faction power: sum credits + XP by faction, count active agents.
	type factionRow struct {
		Faction      string `json:"faction"`
		TotalCredits int64  `json:"total_credits"`
		TotalXP      int64  `json:"total_xp"`
		AgentCount   int64  `json:"agent_count"`
		ShipCount    int64  `json:"ship_count"`
		Power        int64  `json:"power"` // composite score
	}

	factionRows, err := pool.Query(ctx,
		`SELECT a.faction,
		        SUM(a.credits)    AS total_credits,
		        SUM(a.experience) AS total_xp,
		        COUNT(a.id)       AS agent_count,
		        COALESCE((SELECT COUNT(*) FROM ships s
		                  JOIN agents a2 ON a2.id = s.agent_id
		                  WHERE a2.faction = a.faction AND s.hull_points > 0), 0) AS ship_count
		 FROM agents a
		 WHERE a.banned_at IS NULL
		 GROUP BY a.faction
		 ORDER BY SUM(a.credits) + SUM(a.experience) DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "faction query failed")
		return
	}
	defer factionRows.Close()

	var factions []factionRow
	for factionRows.Next() {
		var f factionRow
		if err := factionRows.Scan(&f.Faction, &f.TotalCredits, &f.TotalXP,
			&f.AgentCount, &f.ShipCount); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		f.Power = f.TotalCredits + f.TotalXP*10
		factions = append(factions, f)
	}
	if factions == nil {
		factions = []factionRow{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"top_credits": topCredits,
		"top_xp":      topXP,
		"factions":    factions,
	})
}
