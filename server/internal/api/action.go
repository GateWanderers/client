package api

import (
	"encoding/json"
	"net/http"
)

type actionRequest struct {
	Type       string          `json:"type"`
	Parameters json.RawMessage `json:"parameters"`
}

var validActionTypes = map[string]bool{
	"EXPLORE":      true,
	"DIAL_GATE":    true,
	"ATTACK":       true,
	"GATHER":       true,
	"BUY":          true,
	"SELL":         true,
	"ACCEPT_TRADE": true,
	"RESEARCH":     true,
	"DIPLOMACY":    true,
	"REPAIR":       true,
	"UPGRADE":      true,
	"BUY_SHIP":     true,
	"DEFEND":       true,
}

// handleAgentAction queues an action for the next tick.
// POST /agent/action (auth required)
func (s *Server) handleAgentAction(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if !validActionTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "unsupported action type; valid: EXPLORE, DIAL_GATE, ATTACK, GATHER, BUY, SELL, ACCEPT_TRADE, RESEARCH, DIPLOMACY, REPAIR, UPGRADE, BUY_SHIP, DEFEND")
		return
	}

	// Default parameters to empty JSON object if not provided.
	params := req.Parameters
	if len(params) == 0 {
		params = json.RawMessage("{}")
	}

	// Resolve agent_id and current ship location from the authenticated account.
	var agentID, galaxyID string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT a.id, s.galaxy_id
		 FROM agents a
		 JOIN ships s ON s.agent_id = a.id
		 WHERE a.account_id = $1
		 ORDER BY s.created_at ASC LIMIT 1`,
		accountID,
	).Scan(&agentID, &galaxyID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent or ship not found")
		return
	}

	// Upsert into tick_queue.
	_, err = s.registry.Pool().Exec(r.Context(),
		`INSERT INTO tick_queue (agent_id, galaxy_id, action_type, parameters)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (agent_id) DO UPDATE
		   SET galaxy_id    = EXCLUDED.galaxy_id,
		       action_type  = EXCLUDED.action_type,
		       parameters   = EXCLUDED.parameters,
		       submitted_at = NOW()`,
		agentID, galaxyID, req.Type, params,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to queue action")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"queued": true,
		"action": req.Type,
	})
}
