package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gorilla/websocket"
)

// generateID returns a random 8-byte hex string for anonymous session keys.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for development
	},
}

// handleStream upgrades the connection to WebSocket and streams game events.
// GET /stream
// Auth via ?token=<paseto> query param OR Authorization: Bearer header.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	// Resolve token from query param or Authorization header.
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = parts[1]
		}
	}

	// Determine accountID: authenticated clients get their real accountID,
	// anonymous map viewers get a generated key so they still receive broadcasts.
	var accountID string
	if token != "" {
		id, err := s.tokenMaker.VerifyToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		accountID = id
	} else {
		// Anonymous connection — generate a throwaway key.
		accountID = "anon:" + generateID()
	}

	// Upgrade HTTP → WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("stream: upgrade error", "err", err)
		return
	}
	defer conn.Close()

	// Fetch current tick number.
	var currentTick int64
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT tick_number FROM tick_state WHERE id = 1`,
	).Scan(&currentTick)

	// Register with hub.
	ch := s.hub.Register(accountID)
	defer s.hub.Unregister(accountID, ch)

	// Send connected message.
	if err := conn.WriteJSON(map[string]interface{}{
		"type": "connected",
		"tick": currentTick,
	}); err != nil {
		return
	}

	// Pump messages from hub channel to WebSocket.
	// Run a goroutine to handle client-side closes (reads).
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				slog.Error("stream: reader panic recovered",
					"account", accountID,
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		for {
			// We don't process inbound messages, just drain them so the
			// connection stays alive and we detect disconnects.
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(msg); err != nil {
				slog.Error("stream: write error", "account", accountID, "err", err)
				return
			}
		}
	}
}
