package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
)

// ── Models ────────────────────────────────────────────────────────────────────

type clanRow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Tag         string    `json:"tag"`
	Description string    `json:"description"`
	LeaderID    string    `json:"leader_agent_id"`
	LeaderName  string    `json:"leader_name"`
	Treasury    int64     `json:"treasury"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type clanMember struct {
	AgentID   string    `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	Faction   string    `json:"faction"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// agentIDForAccount returns the agent's UUID for the given account_id.
func (s *Server) agentIDForAccount(r *http.Request, accountID string) (string, bool) {
	var agentID string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT id FROM agents WHERE account_id = $1 AND status != 'banned'`,
		accountID,
	).Scan(&agentID)
	return agentID, err == nil
}

// clanIDForAgent returns the clan_id the agent belongs to, or "" if none.
func (s *Server) clanIDForAgent(r *http.Request, agentID string) string {
	var clanID string
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT clan_id FROM clan_members WHERE agent_id = $1`,
		agentID,
	).Scan(&clanID)
	return clanID
}

// ── POST /clan ────────────────────────────────────────────────────────────────

func (s *Server) handleCreateClan(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Agent must not already be in a clan.
	if s.clanIDForAgent(r, agentID) != "" {
		writeError(w, http.StatusConflict, "already in a clan — leave first")
		return
	}

	var body struct {
		Name        string `json:"name"`
		Tag         string `json:"tag"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Tag = strings.TrimSpace(strings.ToUpper(body.Tag))
	body.Description = strings.TrimSpace(body.Description)

	if utf8.RuneCountInString(body.Name) < 3 || utf8.RuneCountInString(body.Name) > 32 {
		writeError(w, http.StatusBadRequest, "name must be 3–32 characters")
		return
	}
	if utf8.RuneCountInString(body.Tag) < 2 || utf8.RuneCountInString(body.Tag) > 5 {
		writeError(w, http.StatusBadRequest, "tag must be 2–5 characters")
		return
	}

	// Insert clan + leader membership atomically.
	tx, err := s.registry.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	var clanID string
	err = tx.QueryRow(r.Context(),
		`INSERT INTO clans (name, tag, description, leader_agent_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		body.Name, body.Tag, body.Description, agentID,
	).Scan(&clanID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "clan name already taken")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to create clan")
		}
		return
	}

	_, err = tx.Exec(r.Context(),
		`INSERT INTO clan_members (clan_id, agent_id, role) VALUES ($1, $2, 'leader')`,
		clanID, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add leader to clan")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": clanID})
}

// ── GET /clan/{clanID} ────────────────────────────────────────────────────────

func (s *Server) handleGetClan(w http.ResponseWriter, r *http.Request) {
	clanID := chi.URLParam(r, "clanID")

	var clan clanRow
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT c.id, c.name, c.tag, c.description, c.leader_agent_id,
		        a.name, c.treasury, c.created_at,
		        (SELECT COUNT(*) FROM clan_members WHERE clan_id = c.id)
		 FROM clans c
		 JOIN agents a ON a.id = c.leader_agent_id
		 WHERE c.id = $1`,
		clanID,
	).Scan(&clan.ID, &clan.Name, &clan.Tag, &clan.Description,
		&clan.LeaderID, &clan.LeaderName, &clan.Treasury, &clan.CreatedAt,
		&clan.MemberCount)
	if err != nil {
		writeError(w, http.StatusNotFound, "clan not found")
		return
	}

	// Members list.
	rows, err := s.registry.Pool().Query(r.Context(),
		`SELECT cm.agent_id, a.name, a.faction, cm.role, cm.joined_at
		 FROM clan_members cm
		 JOIN agents a ON a.id = cm.agent_id
		 WHERE cm.clan_id = $1
		 ORDER BY cm.role DESC, cm.joined_at ASC`,
		clanID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch members")
		return
	}
	defer rows.Close()

	members := []clanMember{}
	for rows.Next() {
		var m clanMember
		if err := rows.Scan(&m.AgentID, &m.AgentName, &m.Faction, &m.Role, &m.JoinedAt); err == nil {
			members = append(members, m)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"clan":    clan,
		"members": members,
	})
}

// ── GET /clan/mine ────────────────────────────────────────────────────────────

func (s *Server) handleGetMyClan(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	clanID := s.clanIDForAgent(r, agentID)
	if clanID == "" {
		writeError(w, http.StatusNotFound, "not in a clan")
		return
	}

	// Reuse the GET /clan/{id} logic by forwarding.
	chi.RouteContext(r.Context()).URLParams.Add("clanID", clanID)
	s.handleGetClan(w, r)
}

// ── POST /clan/{clanID}/invite ─────────────────────────────────────────────────

func (s *Server) handleClanInvite(w http.ResponseWriter, r *http.Request) {
	clanID := chi.URLParam(r, "clanID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Requester must be leader or officer.
	var role string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT role FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, agentID,
	).Scan(&role)
	if err != nil || (role != "leader" && role != "officer") {
		writeError(w, http.StatusForbidden, "only the clan leader or officer can invite")
		return
	}

	var body struct {
		AgentName string `json:"agent_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.AgentName == "" {
		writeError(w, http.StatusBadRequest, "agent_name required")
		return
	}

	// Find target agent.
	var targetID string
	err = s.registry.Pool().QueryRow(r.Context(),
		`SELECT id FROM agents WHERE name = $1 AND status = 'active'`,
		body.AgentName,
	).Scan(&targetID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found or inactive")
		return
	}

	// Target must not already be in a clan.
	if s.clanIDForAgent(r, targetID) != "" {
		writeError(w, http.StatusConflict, "agent is already in a clan")
		return
	}

	_, err = s.registry.Pool().Exec(r.Context(),
		`INSERT INTO clan_members (clan_id, agent_id, role) VALUES ($1, $2, 'member')
		 ON CONFLICT DO NOTHING`,
		clanID, targetID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── DELETE /clan/{clanID}/member/{agentID} ────────────────────────────────────

func (s *Server) handleClanKick(w http.ResponseWriter, r *http.Request) {
	clanID := chi.URLParam(r, "clanID")
	targetAgentID := chi.URLParam(r, "agentID")
	accountID := accountIDFromContext(r.Context())
	actorID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Actor must be leader.
	var role string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT role FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, actorID,
	).Scan(&role)
	if err != nil || role != "leader" {
		writeError(w, http.StatusForbidden, "only the clan leader can kick members")
		return
	}

	// Cannot kick yourself (use /clan/{id}/leave instead).
	if targetAgentID == actorID {
		writeError(w, http.StatusBadRequest, "use DELETE /clan/{id}/leave to leave")
		return
	}

	_, err = s.registry.Pool().Exec(r.Context(),
		`DELETE FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, targetAgentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── DELETE /clan/{clanID}/leave ───────────────────────────────────────────────

func (s *Server) handleClanLeave(w http.ResponseWriter, r *http.Request) {
	clanID := chi.URLParam(r, "clanID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Check if the agent is the leader.
	var role string
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT role FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, agentID,
	).Scan(&role)
	if role == "" {
		writeError(w, http.StatusNotFound, "not a member of this clan")
		return
	}

	if role == "leader" {
		// Promote oldest officer or member before leaving, or disband if last.
		var nextLeaderID string
		_ = s.registry.Pool().QueryRow(r.Context(),
			`SELECT agent_id FROM clan_members
			 WHERE clan_id = $1 AND agent_id != $2
			 ORDER BY CASE role WHEN 'officer' THEN 1 ELSE 2 END, joined_at ASC
			 LIMIT 1`,
			clanID, agentID,
		).Scan(&nextLeaderID)

		if nextLeaderID == "" {
			// Last member — disband clan.
			_, _ = s.registry.Pool().Exec(r.Context(),
				`DELETE FROM clans WHERE id = $1`, clanID)
			writeJSON(w, http.StatusOK, map[string]string{"result": "clan_disbanded"})
			return
		}

		tx, err := s.registry.Pool().Begin(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db error")
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		_, _ = tx.Exec(r.Context(),
			`UPDATE clans SET leader_agent_id = $1 WHERE id = $2`,
			nextLeaderID, clanID)
		_, _ = tx.Exec(r.Context(),
			`UPDATE clan_members SET role = 'leader' WHERE clan_id = $1 AND agent_id = $2`,
			clanID, nextLeaderID)
		_, _ = tx.Exec(r.Context(),
			`DELETE FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
			clanID, agentID)

		if err := tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "transaction failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": "left_promoted_new_leader"})
		return
	}

	_, err := s.registry.Pool().Exec(r.Context(),
		`DELETE FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to leave clan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── DELETE /clan/{clanID} (disband — leader only) ─────────────────────────────

func (s *Server) handleDisbandClan(w http.ResponseWriter, r *http.Request) {
	clanID := chi.URLParam(r, "clanID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var role string
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT role FROM clan_members WHERE clan_id = $1 AND agent_id = $2`,
		clanID, agentID,
	).Scan(&role)
	if role != "leader" {
		writeError(w, http.StatusForbidden, "only the clan leader can disband the clan")
		return
	}

	_, err := s.registry.Pool().Exec(r.Context(),
		`DELETE FROM clans WHERE id = $1`, clanID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disband clan")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
