package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gatewanderers/server/internal/registry"
	"gatewanderers/server/internal/research"
)

// handleAgentState returns the full agent + ship state for the authenticated account.
func (s *Server) handleAgentState(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	agent, ship, err := s.registry.GetAgentState(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch agent state")
		return
	}

	alliances, err := s.registry.GetAlliances(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch alliances")
		return
	}

	// Parse completed research to compute active bonuses.
	var completedResearch []string
	_ = json.Unmarshal(agent.Research, &completedResearch)
	weaponBonus, shieldBonus := research.CombatBonuses(completedResearch)

	// Fetch clan info if the agent is a member.
	var clanInfo *registry.ClanInfo
	var clanID, clanName, clanTag, clanRole string
	err = s.registry.Pool().QueryRow(r.Context(),
		`SELECT c.id, c.name, c.tag, cm.role
		 FROM clan_members cm
		 JOIN clans c ON c.id = cm.clan_id
		 WHERE cm.agent_id = $1`,
		agent.ID,
	).Scan(&clanID, &clanName, &clanTag, &clanRole)
	if err == nil {
		clanInfo = &registry.ClanInfo{
			ID:   clanID,
			Name: clanName,
			Tag:  clanTag,
			Role: clanRole,
		}
	}

	writeJSON(w, http.StatusOK, registry.AgentState{
		Agent:     agent,
		Ship:      ship,
		Alliances: alliances,
		Clan:      clanInfo,
		ResearchBonuses: registry.ResearchBonuses{
			WeaponBonus: weaponBonus,
			ShieldBonus: shieldBonus,
			GatherBonus: research.GatherBonus(completedResearch),
			TradeBonus:  research.TradeBonus(completedResearch),
		},
	})
}

// handleMissionBrief updates the agent's mission brief.
// PUT /agent/mission-brief
func (s *Server) handleMissionBrief(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		MissionBrief string `json:"mission_brief"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	_, err := s.registry.Pool().Exec(r.Context(),
		`UPDATE agents SET mission_brief = $1
		 WHERE account_id = $2`,
		body.MissionBrief, accountID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mission brief")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleVeto registers a player veto (max once per hour, server-enforced).
// POST /agent/veto
func (s *Server) handleVeto(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var lastVetoAt *time.Time
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT last_veto_at FROM agents WHERE account_id = $1`, accountID,
	).Scan(&lastVetoAt)

	if lastVetoAt != nil && time.Since(*lastVetoAt) < time.Hour {
		remaining := time.Hour - time.Since(*lastVetoAt)
		writeError(w, http.StatusTooManyRequests,
			"veto on cooldown; available in "+remaining.Round(time.Second).String())
		return
	}

	_, err := s.registry.Pool().Exec(r.Context(),
		`UPDATE agents SET last_veto_at = NOW() WHERE account_id = $1`, accountID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record veto")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":             true,
		"next_veto_at":   time.Now().Add(time.Hour),
	})
}

// handleOverride registers a player emergency override (max once per day, server-enforced).
// POST /agent/override
func (s *Server) handleOverride(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var lastOverrideAt *time.Time
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT last_override_at FROM agents WHERE account_id = $1`, accountID,
	).Scan(&lastOverrideAt)

	if lastOverrideAt != nil && time.Since(*lastOverrideAt) < 24*time.Hour {
		remaining := 24*time.Hour - time.Since(*lastOverrideAt)
		writeError(w, http.StatusTooManyRequests,
			"override on cooldown; available in "+remaining.Round(time.Second).String())
		return
	}

	var body struct {
		Action     string          `json:"action"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Record override timestamp.
	_, err := s.registry.Pool().Exec(r.Context(),
		`UPDATE agents SET last_override_at = NOW() WHERE account_id = $1`, accountID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record override")
		return
	}

	// If an action was specified, queue it immediately (overwriting any existing queued action).
	if body.Action != "" {
		params := body.Parameters
		if len(params) == 0 {
			params = json.RawMessage("{}")
		}
		var agentID, galaxyID string
		if err := s.registry.Pool().QueryRow(r.Context(),
			`SELECT a.id, s.galaxy_id FROM agents a
			 JOIN ships s ON s.agent_id = a.id
			 WHERE a.account_id = $1 ORDER BY s.created_at ASC LIMIT 1`,
			accountID,
		).Scan(&agentID, &galaxyID); err == nil {
			_, _ = s.registry.Pool().Exec(r.Context(),
				`INSERT INTO tick_queue (agent_id, galaxy_id, action_type, parameters)
				 VALUES ($1, $2, $3, $4)
				 ON CONFLICT (agent_id) DO UPDATE
				   SET galaxy_id = EXCLUDED.galaxy_id,
				       action_type = EXCLUDED.action_type,
				       parameters = EXCLUDED.parameters,
				       submitted_at = NOW()`,
				agentID, galaxyID, body.Action, params,
			)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                 true,
		"next_override_at":   time.Now().Add(24 * time.Hour),
	})
}
