package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"gatewanderers/server/internal/hub"
)

// ── Admin Middleware ────────────────────────────────────────────────────────

// adminMiddleware requires the authenticated account to have is_admin = true.
func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accountID := accountIDFromContext(r.Context())
		if accountID == "" {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		var isAdmin bool
		err := s.registry.Pool().QueryRow(r.Context(),
			`SELECT is_admin FROM accounts WHERE id = $1`, accountID,
		).Scan(&isAdmin)
		if err != nil || !isAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Admin Broadcast ────────────────────────────────────────────────────────

// adminBroadcast broadcasts an admin_event to all connected WS clients.
// Regular game clients receive the message but ignore unknown event types.
func (s *Server) adminBroadcast(action string, details map[string]interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"action": action,
		"data":   details,
	})
	s.hub.Broadcast(hub.Message{
		Type:  "admin_event",
		Event: json.RawMessage(payload),
	})
}

// ── Audit Helper ───────────────────────────────────────────────────────────

func (s *Server) auditLog(ctx context.Context, adminAccountID, action, targetID, details string) {
	if details == "" {
		details = "null"
	}
	_, _ = s.registry.Pool().Exec(ctx,
		`INSERT INTO admin_audit_log (admin_account_id, action, target_id, details)
		 VALUES ($1, $2, NULLIF($3,''), $4::jsonb)`,
		adminAccountID, action, targetID, details,
	)
}

// ── GET /admin/health ──────────────────────────────────────────────────────

func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pool := s.registry.Pool()

	// DB ping latency.
	dbStart := time.Now()
	dbOK := false
	var dbPing bool
	if err := pool.QueryRow(ctx, `SELECT true`).Scan(&dbPing); err == nil {
		dbOK = true
	}
	dbLatencyMs := time.Since(dbStart).Milliseconds()

	// Tick state from DB.
	var lastDBTickAt time.Time
	var dbTickNumber int64
	_ = pool.QueryRow(ctx,
		`SELECT tick_number, last_tick_at FROM tick_state WHERE id = 1`,
	).Scan(&dbTickNumber, &lastDBTickAt)

	ts := s.ticker.Stats()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"db_ok":             dbOK,
		"db_latency_ms":     dbLatencyMs,
		"ws_clients":        s.hub.ClientCount(),
		"ticker_paused":     ts.Paused,
		"ticker_count":      ts.TickCount,
		"ticker_interval_s": ts.Interval.Seconds(),
		"ticker_started_at": ts.StartedAt,
		"ticker_last_tick":  ts.LastTickAt,
		"uptime_sec":        ts.UptimeSec,
		"db_tick_number":    dbTickNumber,
		"db_last_tick_at":   lastDBTickAt,
	})
}

// ── GET /admin/stats ───────────────────────────────────────────────────────

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pool := s.registry.Pool()

	queries := []struct {
		key string
		sql string
	}{
		{"total_accounts", `SELECT COUNT(*) FROM accounts`},
		{"total_agents", `SELECT COUNT(*) FROM agents`},
		{"banned_agents", `SELECT COUNT(*) FROM agents WHERE banned_at IS NOT NULL`},
		{"total_ships", `SELECT COUNT(*) FROM ships`},
		{"active_alliances", `SELECT COUNT(*) FROM alliances WHERE status = 'active'`},
		{"events_24h", `SELECT COUNT(*) FROM events WHERE created_at > NOW() - INTERVAL '24 hours'`},
		{"world_events_24h", `SELECT COUNT(*) FROM world_events WHERE created_at > NOW() - INTERVAL '24 hours'`},
		{"chat_messages_24h", `SELECT COUNT(*) FROM chat_messages WHERE created_at > NOW() - INTERVAL '24 hours'`},
		{"open_reports", `SELECT COUNT(*) FROM chat_reports`},
		{"tick_queue_depth", `SELECT COUNT(*) FROM tick_queue`},
	}

	result := make(map[string]interface{})
	for _, q := range queries {
		var n int64
		_ = pool.QueryRow(ctx, q.sql).Scan(&n)
		result[q.key] = n
	}
	result["ws_clients"] = s.hub.ClientCount()
	writeJSON(w, http.StatusOK, result)
}

// ── GET /admin/clients ─────────────────────────────────────────────────────

func (s *Server) handleAdminClients(w http.ResponseWriter, r *http.Request) {
	ids := s.hub.ConnectedIDs()
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"count":   len(ids),
		"clients": ids,
	})
}

// ── GET /admin/audit ───────────────────────────────────────────────────────

func (s *Server) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.registry.Pool().Query(ctx,
		`SELECT l.id, l.admin_account_id, a.email, l.action, l.target_id, l.details, l.created_at
		 FROM admin_audit_log l
		 JOIN accounts a ON a.id = l.admin_account_id
		 ORDER BY l.created_at DESC
		 LIMIT 200`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch audit log")
		return
	}
	defer rows.Close()

	type entry struct {
		ID             string      `json:"id"`
		AdminAccountID string      `json:"admin_account_id"`
		AdminEmail     string      `json:"admin_email"`
		Action         string      `json:"action"`
		TargetID       *string     `json:"target_id,omitempty"`
		Details        interface{} `json:"details,omitempty"`
		CreatedAt      time.Time   `json:"created_at"`
	}

	var entries []entry
	for rows.Next() {
		var e entry
		var targetID *string
		var details []byte
		if err := rows.Scan(&e.ID, &e.AdminAccountID, &e.AdminEmail, &e.Action, &targetID, &details, &e.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		e.TargetID = targetID
		if len(details) > 0 && string(details) != "null" {
			e.Details = string(details)
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []entry{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries, "count": len(entries)})
}

// ── GET /admin/agents ──────────────────────────────────────────────────────

func (s *Server) handleAdminAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	search := r.URL.Query().Get("q")

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT ag.id, ac.id, ac.email, ac.is_admin, ag.name, ag.faction,
		        ag.credits, ag.experience, ag.status,
		        ag.banned_at, ag.ban_reason, ag.created_at
		 FROM agents ag
		 JOIN accounts ac ON ac.id = ag.account_id
		 WHERE ($1 = '' OR ag.name ILIKE '%' || $1 || '%' OR ac.email ILIKE '%' || $1 || '%')
		 ORDER BY ag.created_at DESC
		 LIMIT 500`,
		search,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query agents")
		return
	}
	defer rows.Close()

	type agentRow struct {
		AgentID    string     `json:"agent_id"`
		AccountID  string     `json:"account_id"`
		Email      string     `json:"email"`
		IsAdmin    bool       `json:"is_admin"`
		Name       string     `json:"name"`
		Faction    string     `json:"faction"`
		Credits    int64      `json:"credits"`
		Experience int64      `json:"experience"`
		Status     string     `json:"status"`
		BannedAt   *time.Time `json:"banned_at,omitempty"`
		BanReason  *string    `json:"ban_reason,omitempty"`
		CreatedAt  time.Time  `json:"created_at"`
	}

	var agents []agentRow
	for rows.Next() {
		var a agentRow
		if err := rows.Scan(
			&a.AgentID, &a.AccountID, &a.Email, &a.IsAdmin,
			&a.Name, &a.Faction, &a.Credits, &a.Experience, &a.Status,
			&a.BannedAt, &a.BanReason, &a.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []agentRow{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"agents": agents, "count": len(agents)})
}

// ── POST /admin/agents/{agentID}/ban ──────────────────────────────────────

func (s *Server) handleAdminBanAgent(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	agentID := chi.URLParam(r, "agentID")
	ctx := r.Context()

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	tag, err := s.registry.Pool().Exec(ctx,
		`UPDATE agents SET banned_at = NOW(), ban_reason = $1 WHERE id = $2`,
		body.Reason, agentID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Also kick their active WS connection.
	var accountID string
	_ = s.registry.Pool().QueryRow(ctx,
		`SELECT account_id FROM agents WHERE id = $1`, agentID,
	).Scan(&accountID)
	if accountID != "" {
		s.hub.KickClient(accountID)
	}

	reason := strings.ReplaceAll(body.Reason, `"`, `\"`)
	s.auditLog(ctx, adminID, "ban_agent", agentID, `{"reason":"`+reason+`"}`)
	writeJSON(w, http.StatusOK, map[string]string{"status": "banned"})
}

// ── DELETE /admin/agents/{agentID}/ban ────────────────────────────────────

func (s *Server) handleAdminUnbanAgent(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	agentID := chi.URLParam(r, "agentID")
	ctx := r.Context()

	tag, err := s.registry.Pool().Exec(ctx,
		`UPDATE agents SET banned_at = NULL, ban_reason = NULL WHERE id = $1`, agentID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	s.auditLog(ctx, adminID, "unban_agent", agentID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbanned"})
}

// ── POST /admin/agents/{agentID}/kick ─────────────────────────────────────

func (s *Server) handleAdminKickAgent(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	agentID := chi.URLParam(r, "agentID")
	ctx := r.Context()

	var accountID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT account_id FROM agents WHERE id = $1`, agentID,
	).Scan(&accountID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	s.hub.KickClient(accountID)
	s.auditLog(ctx, adminID, "kick_agent", agentID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "kicked"})
}

// ── GET /admin/chat/reports ────────────────────────────────────────────────

func (s *Server) handleAdminChatReports(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.registry.Pool().Query(ctx,
		`SELECT cr.id, cr.reporter_id, reporter.name, cr.message_id,
		        cm.channel, cm.content, sender.name, cm.created_at,
		        cr.reason, cr.created_at
		 FROM chat_reports cr
		 JOIN agents reporter ON reporter.id = cr.reporter_id
		 JOIN chat_messages cm ON cm.id = cr.message_id
		 JOIN agents sender ON sender.id = cm.sender_id
		 ORDER BY cr.created_at DESC
		 LIMIT 200`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch reports")
		return
	}
	defer rows.Close()

	type report struct {
		ReportID     string    `json:"report_id"`
		ReporterID   string    `json:"reporter_id"`
		ReporterName string    `json:"reporter_name"`
		MessageID    string    `json:"message_id"`
		Channel      string    `json:"channel"`
		Content      string    `json:"content"`
		SenderName   string    `json:"sender_name"`
		MessageAt    time.Time `json:"message_at"`
		Reason       *string   `json:"reason,omitempty"`
		ReportedAt   time.Time `json:"reported_at"`
	}

	var reports []report
	for rows.Next() {
		var rp report
		if err := rows.Scan(
			&rp.ReportID, &rp.ReporterID, &rp.ReporterName,
			&rp.MessageID, &rp.Channel, &rp.Content, &rp.SenderName, &rp.MessageAt,
			&rp.Reason, &rp.ReportedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		reports = append(reports, rp)
	}
	if reports == nil {
		reports = []report{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"reports": reports, "count": len(reports)})
}

// ── DELETE /admin/chat/messages/{msgID} ───────────────────────────────────

func (s *Server) handleAdminDeleteMessage(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	msgID := chi.URLParam(r, "msgID")
	ctx := r.Context()

	tag, err := s.registry.Pool().Exec(ctx,
		`DELETE FROM chat_messages WHERE id = $1`, msgID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}

	s.auditLog(ctx, adminID, "delete_message", msgID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── POST /admin/accounts/{accountID}/promote ──────────────────────────────

func (s *Server) handleAdminPromote(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	accountID := chi.URLParam(r, "accountID")
	ctx := r.Context()

	tag, err := s.registry.Pool().Exec(ctx,
		`UPDATE accounts SET is_admin = true WHERE id = $1`, accountID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	s.auditLog(ctx, adminID, "promote_admin", accountID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

// ── DELETE /admin/accounts/{accountID}/promote ────────────────────────────

func (s *Server) handleAdminDemote(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	accountID := chi.URLParam(r, "accountID")
	ctx := r.Context()

	// Prevent self-demote.
	if accountID == adminID {
		writeError(w, http.StatusBadRequest, "cannot demote yourself")
		return
	}

	tag, err := s.registry.Pool().Exec(ctx,
		`UPDATE accounts SET is_admin = false WHERE id = $1`, accountID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	s.auditLog(ctx, adminID, "demote_admin", accountID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "demoted"})
}

// ── POST /admin/tick/force ─────────────────────────────────────────────────

func (s *Server) handleAdminForceTick(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	s.ticker.ForceTick()
	s.auditLog(r.Context(), adminID, "force_tick", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "tick queued"})
}

// ── POST /admin/tick/pause ────────────────────────────────────────────────

func (s *Server) handleAdminPauseTick(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	s.ticker.Pause()
	s.auditLog(r.Context(), adminID, "pause_ticker", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

// ── POST /admin/tick/resume ───────────────────────────────────────────────

func (s *Server) handleAdminResumeTick(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	s.ticker.Resume()
	s.auditLog(r.Context(), adminID, "resume_ticker", "", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}
