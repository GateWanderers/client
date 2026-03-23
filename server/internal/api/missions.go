package api

import (
	"net/http"
)

// handleAgentMissions serves GET /agent/missions — returns the agent's active and recent missions.
func (s *Server) handleAgentMissions(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()
	pool := s.registry.Pool()

	var agentID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1`, accountID,
	).Scan(&agentID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	rows, err := pool.Query(ctx,
		`SELECT id, type, title_en, title_de, desc_en, desc_de,
		        target_resource, target_quantity, progress,
		        reward_credits, reward_xp, status, expires_at_tick
		 FROM missions
		 WHERE agent_id = $1
		 ORDER BY created_at DESC
		 LIMIT 20`,
		agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch missions")
		return
	}
	defer rows.Close()

	type missionEntry struct {
		ID             string  `json:"id"`
		Type           string  `json:"type"`
		TitleEN        string  `json:"title_en"`
		TitleDE        string  `json:"title_de"`
		DescEN         string  `json:"desc_en"`
		DescDE         string  `json:"desc_de"`
		TargetResource *string `json:"target_resource"`
		TargetQuantity int     `json:"target_quantity"`
		Progress       int     `json:"progress"`
		RewardCredits  int     `json:"reward_credits"`
		RewardXP       int     `json:"reward_xp"`
		Status         string  `json:"status"`
		ExpiresAtTick  int64   `json:"expires_at_tick"`
	}

	var missionList []missionEntry
	for rows.Next() {
		var m missionEntry
		if err := rows.Scan(
			&m.ID, &m.Type, &m.TitleEN, &m.TitleDE, &m.DescEN, &m.DescDE,
			&m.TargetResource, &m.TargetQuantity, &m.Progress,
			&m.RewardCredits, &m.RewardXP, &m.Status, &m.ExpiresAtTick,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan mission")
			return
		}
		missionList = append(missionList, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "mission rows error")
		return
	}
	if missionList == nil {
		missionList = []missionEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"missions": missionList,
	})
}
