package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"gatewanderers/server/internal/mining"
)

// handleGalaxyNodes returns mining node info for a system.
// Richness/reserves are only revealed if the requesting agent has an active survey.
func (s *Server) handleGalaxyNodes(w http.ResponseWriter, r *http.Request) {
	systemID := chi.URLParam(r, "systemID")
	if systemID == "" {
		writeError(w, http.StatusBadRequest, "missing systemID")
		return
	}

	ctx := r.Context()

	// Determine agent (optional — unauthenticated users get redacted data).
	accountID := accountIDFromContext(ctx)
	agentID := ""
	if accountID != "" {
		_ = s.registry.Pool().QueryRow(ctx,
			`SELECT id FROM agents WHERE account_id = $1 LIMIT 1`, accountID,
		).Scan(&agentID)
	}

	// Check for active survey.
	surveyed := false
	if agentID != "" {
		var dummy int
		err := s.registry.Pool().QueryRow(ctx,
			`SELECT 1 FROM surveys
			 WHERE agent_id = $1 AND system_id = $2
			   AND expires_at_tick >= (SELECT tick_number FROM tick_state WHERE id = 1)`,
			agentID, systemID,
		).Scan(&dummy)
		surveyed = err == nil
	}

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT resource_type, richness, current_reserves, max_reserves, regen_per_tick
		 FROM mining_nodes WHERE system_id = $1`,
		systemID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type nodeItem struct {
		ResourceType    string `json:"resource_type"`
		Richness        string `json:"richness,omitempty"`
		CurrentReserves *int   `json:"current_reserves,omitempty"`
		MaxReserves     *int   `json:"max_reserves,omitempty"`
		RegenPerTick    *int   `json:"regen_per_tick,omitempty"`
		Surveyed        bool   `json:"surveyed"`
	}

	var nodes []nodeItem
	for rows.Next() {
		var resourceType, richness string
		var cur, max, regen int
		if err := rows.Scan(&resourceType, &richness, &cur, &max, &regen); err != nil {
			continue
		}
		node := nodeItem{ResourceType: resourceType, Surveyed: surveyed}
		if surveyed {
			node.Richness = richness
			node.CurrentReserves = &cur
			node.MaxReserves = &max
			node.RegenPerTick = &regen
		}
		nodes = append(nodes, node)
	}
	if nodes == nil {
		nodes = []nodeItem{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"system_id": systemID,
		"surveyed":  surveyed,
		"nodes":     nodes,
	})
}

// handleAgentSurveys returns active surveys for the authenticated agent.
func (s *Server) handleAgentSurveys(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()
	var agentID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1 LIMIT 1`, accountID,
	).Scan(&agentID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT sv.system_id, sv.surveyed_at_tick, sv.expires_at_tick,
		        (sv.expires_at_tick - ts.tick_number) AS ticks_remaining
		 FROM surveys sv, tick_state ts
		 WHERE sv.agent_id = $1 AND ts.id = 1 AND sv.expires_at_tick >= ts.tick_number
		 ORDER BY sv.expires_at_tick DESC`,
		agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type surveyItem struct {
		SystemID       string `json:"system_id"`
		SurveyedAtTick int64  `json:"surveyed_at_tick"`
		ExpiresAtTick  int64  `json:"expires_at_tick"`
		TicksRemaining int64  `json:"ticks_remaining"`
	}
	var surveys []surveyItem
	for rows.Next() {
		var sv surveyItem
		if err := rows.Scan(&sv.SystemID, &sv.SurveyedAtTick, &sv.ExpiresAtTick, &sv.TicksRemaining); err != nil {
			continue
		}
		surveys = append(surveys, sv)
	}
	if surveys == nil {
		surveys = []surveyItem{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"surveys": surveys})
}

// handleAgentSkills returns skill levels, XP and cooldown state for the authenticated agent.
func (s *Server) handleAgentSkills(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	ctx := r.Context()
	var agentID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1 LIMIT 1`, accountID,
	).Scan(&agentID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var tickNumber int64
	_ = s.registry.Pool().QueryRow(ctx, `SELECT tick_number FROM tick_state WHERE id = 1`).Scan(&tickNumber)

	// Load existing skill rows for this agent.
	rows, err := s.registry.Pool().Query(ctx,
		`SELECT skill_id, level, xp, cooldown_expires_tick FROM agent_skills WHERE agent_id = $1`,
		agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type skillItem struct {
		SkillID         string `json:"skill_id"`
		Name            string `json:"name"`
		NameDE          string `json:"name_de"`
		Description     string `json:"description"`
		DescriptionDE   string `json:"description_de"`
		Level           int    `json:"level"`
		XP              int    `json:"xp"`
		XPToNextLevel   int    `json:"xp_to_next_level"`
		CooldownExpires int64  `json:"cooldown_expires_tick"`
		TicksOnCooldown int64  `json:"ticks_on_cooldown"`
		Ready           bool   `json:"ready"`
	}

	learned := make(map[string]skillItem)
	for rows.Next() {
		var skillID string
		var level, xp int
		var cooldown int64
		if err := rows.Scan(&skillID, &level, &xp, &cooldown); err != nil {
			continue
		}
		sk, ok := mining.SkillRegistry[skillID]
		if !ok {
			continue
		}
		xpToNext := -1
		if level < 5 {
			xpToNext = sk.LevelThresholds[level-1] - xp
			if xpToNext < 0 {
				xpToNext = 0
			}
		}
		ticks := cooldown - tickNumber
		if ticks < 0 {
			ticks = 0
		}
		learned[skillID] = skillItem{
			SkillID:         skillID,
			Name:            sk.Name,
			NameDE:          sk.NameDE,
			Description:     sk.Description,
			DescriptionDE:   sk.DescriptionDE,
			Level:           level,
			XP:              xp,
			XPToNextLevel:   xpToNext,
			CooldownExpires: cooldown,
			TicksOnCooldown: ticks,
			Ready:           ticks == 0,
		}
	}

	// Return all defined skills — learned ones with real data, others at L1/ready.
	var skills []skillItem
	for id, sk := range mining.SkillRegistry {
		if item, ok := learned[id]; ok {
			skills = append(skills, item)
		} else {
			skills = append(skills, skillItem{
				SkillID:         id,
				Name:            sk.Name,
				NameDE:          sk.NameDE,
				Description:     sk.Description,
				DescriptionDE:   sk.DescriptionDE,
				Level:           1,
				XP:              0,
				XPToNextLevel:   sk.LevelThresholds[0],
				CooldownExpires: 0,
				TicksOnCooldown: 0,
				Ready:           true,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tick":   tickNumber,
		"skills": skills,
	})
}
