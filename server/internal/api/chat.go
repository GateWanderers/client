package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"gatewanderers/server/internal/hub"
)

// validFactions is the set of allowed faction slugs used as channel names.
var validFactions = map[string]bool{
	"tau_ri": true, "free_jaffa": true, "gate_nomad": true,
	"system_lord_remnant": true, "wraith_brood": true, "ancient_seeker": true,
}

// chatMessage is one chat_messages row joined with sender name.
type chatMessage struct {
	ID         string    `json:"id"`
	Channel    string    `json:"channel"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// handleGetChat serves GET /chat/:channel
// global: public; faction channel: auth + matching faction required.
func (s *Server) handleGetChat(w http.ResponseWriter, r *http.Request) {
	channel := chi.URLParam(r, "channel")
	if !isValidChannel(channel) {
		writeError(w, http.StatusBadRequest, "invalid channel; valid: global, tau_ri, free_jaffa, gate_nomad, system_lord_remnant, wraith_brood, ancient_seeker")
		return
	}

	ctx := r.Context()

	if channel != "global" {
		accountID := accountIDFromContext(ctx)
		if accountID == "" {
			writeError(w, http.StatusUnauthorized, "faction chat requires authentication")
			return
		}
		if !s.agentInFaction(ctx, accountID, channel) {
			writeError(w, http.StatusForbidden, "you are not a member of this faction")
			return
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT cm.id, cm.channel, cm.sender_id, a.name, cm.content, cm.created_at
		 FROM chat_messages cm
		 JOIN agents a ON a.id = cm.sender_id
		 WHERE cm.channel = $1
		 ORDER BY cm.created_at DESC
		 LIMIT $2`,
		channel, limit,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch messages")
		return
	}
	defer rows.Close()

	var messages []chatMessage
	for rows.Next() {
		var m chatMessage
		if err := rows.Scan(&m.ID, &m.Channel, &m.SenderID, &m.SenderName, &m.Content, &m.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan message")
			return
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "messages rows error")
		return
	}
	if messages == nil {
		messages = []chatMessage{}
	}
	// Reverse to chronological order (DESC query → reverse for display).
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"channel":  channel,
		"messages": messages,
	})
}

// handlePostChat sends a message.
// POST /chat/:channel  — auth required; faction channels enforce membership.
func (s *Server) handlePostChat(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	channel := chi.URLParam(r, "channel")
	if !isValidChannel(channel) {
		writeError(w, http.StatusBadRequest, "invalid channel")
		return
	}

	ctx := r.Context()

	var agentID, agentName, agentFaction string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id, name, faction FROM agents WHERE account_id = $1`, accountID,
	).Scan(&agentID, &agentName, &agentFaction); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	if channel != "global" && agentFaction != channel {
		writeError(w, http.StatusForbidden, "you are not a member of this faction")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if len([]rune(body.Content)) > 500 {
		writeError(w, http.StatusBadRequest, "message too long (max 500 characters)")
		return
	}

	var msg chatMessage
	if err := s.registry.Pool().QueryRow(ctx,
		`INSERT INTO chat_messages (channel, sender_id, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, channel, sender_id, content, created_at`,
		channel, agentID, body.Content,
	).Scan(&msg.ID, &msg.Channel, &msg.SenderID, &msg.Content, &msg.CreatedAt); err != nil {
		slog.Error("chat: insert message", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to store message")
		return
	}
	msg.SenderName = agentName

	// Build WebSocket payload.
	payload, _ := json.Marshal(map[string]interface{}{
		"id":          msg.ID,
		"channel":     msg.Channel,
		"sender_id":   msg.SenderID,
		"sender_name": msg.SenderName,
		"content":     msg.Content,
		"created_at":  msg.CreatedAt,
	})
	wsMsg := hub.Message{Type: "chat", Event: json.RawMessage(payload)}

	if channel == "global" {
		s.hub.Broadcast(wsMsg)
	} else {
		s.broadcastToFaction(ctx, channel, wsMsg)
	}

	writeJSON(w, http.StatusCreated, msg)
}

// handleMuteAgent mutes an agent's messages (stored server-side, sent to client on login).
// POST /chat/mute   body: {"muted_agent_id":"uuid"}
func (s *Server) handleMuteAgent(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		MutedAgentID string `json:"muted_agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MutedAgentID == "" {
		writeError(w, http.StatusBadRequest, "muted_agent_id is required")
		return
	}
	ctx := r.Context()
	var muterID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1`, accountID,
	).Scan(&muterID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if muterID == body.MutedAgentID {
		writeError(w, http.StatusBadRequest, "you cannot mute yourself")
		return
	}
	_, err := s.registry.Pool().Exec(ctx,
		`INSERT INTO chat_mutes (muter_id, muted_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		muterID, body.MutedAgentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mute agent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleUnmuteAgent removes a mute.
// DELETE /chat/mute   body: {"muted_agent_id":"uuid"}
func (s *Server) handleUnmuteAgent(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		MutedAgentID string `json:"muted_agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MutedAgentID == "" {
		writeError(w, http.StatusBadRequest, "muted_agent_id is required")
		return
	}
	ctx := r.Context()
	var muterID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1`, accountID,
	).Scan(&muterID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	_, _ = s.registry.Pool().Exec(ctx,
		`DELETE FROM chat_mutes WHERE muter_id = $1 AND muted_id = $2`,
		muterID, body.MutedAgentID,
	)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleGetMutes returns the caller's mute list.
// GET /chat/mutes
func (s *Server) handleGetMutes(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	ctx := r.Context()
	var agentID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1`, accountID,
	).Scan(&agentID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT cm.muted_id, a.name FROM chat_mutes cm
		 JOIN agents a ON a.id = cm.muted_id
		 WHERE cm.muter_id = $1`, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch mutes")
		return
	}
	defer rows.Close()

	type muteEntry struct {
		AgentID string `json:"agent_id"`
		Name    string `json:"name"`
	}
	var mutes []muteEntry
	for rows.Next() {
		var m muteEntry
		if err := rows.Scan(&m.AgentID, &m.Name); err == nil {
			mutes = append(mutes, m)
		}
	}
	if mutes == nil {
		mutes = []muteEntry{}
	}
	writeJSON(w, http.StatusOK, mutes)
}

// handleReportMessage files a report for admin review.
// POST /chat/report   body: {"message_id":"uuid","reason":"..."}
func (s *Server) handleReportMessage(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body struct {
		MessageID string `json:"message_id"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MessageID == "" {
		writeError(w, http.StatusBadRequest, "message_id is required")
		return
	}
	ctx := r.Context()
	var reporterID string
	if err := s.registry.Pool().QueryRow(ctx,
		`SELECT id FROM agents WHERE account_id = $1`, accountID,
	).Scan(&reporterID); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	_, err := s.registry.Pool().Exec(ctx,
		`INSERT INTO chat_reports (reporter_id, message_id, reason) VALUES ($1, $2, $3)`,
		reporterID, body.MessageID, body.Reason,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to submit report")
		return
	}

	// Notify admins in real-time.
	var reportCount int64
	_ = s.registry.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM chat_reports`).Scan(&reportCount)
	s.adminBroadcast("new_report", map[string]interface{}{"count": reportCount})

	writeJSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func isValidChannel(ch string) bool {
	return ch == "global" || validFactions[ch]
}

// agentInFaction checks whether the given account's agent belongs to faction.
func (s *Server) agentInFaction(ctx context.Context, accountID, faction string) bool {
	var f string
	err := s.registry.Pool().QueryRow(ctx,
		`SELECT faction FROM agents WHERE account_id = $1`, accountID,
	).Scan(&f)
	return err == nil && f == faction
}

// broadcastToFaction sends a hub message to all accounts whose agents are in faction.
func (s *Server) broadcastToFaction(ctx context.Context, faction string, msg hub.Message) {
	rows, err := s.registry.Pool().Query(ctx,
		`SELECT a.account_id FROM agents a WHERE a.faction = $1`, faction,
	)
	if err != nil {
		slog.Error("chat: broadcastToFaction query", "err", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var accountID string
		if err := rows.Scan(&accountID); err == nil {
			s.hub.SendToAgent(accountID, msg)
		}
	}
}
