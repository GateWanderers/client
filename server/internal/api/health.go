package api

import (
	"net/http"
	"time"
)

// handleHealth returns a lightweight public health check.
// GET /health — no auth required.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Quick DB ping.
	dbOK := false
	var ok bool
	if err := s.registry.Pool().QueryRow(ctx, `SELECT true`).Scan(&ok); err == nil {
		dbOK = true
	}

	status := "ok"
	code := http.StatusOK
	if !dbOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	ts := s.ticker.Stats()

	writeJSON(w, code, map[string]interface{}{
		"status":     status,
		"db_ok":      dbOK,
		"tick":       ts.TickCount,
		"paused":     ts.Paused,
		"uptime_sec": ts.UptimeSec,
		"time":       time.Now().UTC(),
	})
}
