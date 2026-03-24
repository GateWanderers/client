package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── GET /bounties ─────────────────────────────────────────────────────────

func (s *Server) handleGetBounties(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.registry.Pool().Query(ctx,
		`SELECT b.id, b.amount, b.created_at, b.expires_at,
		        placer.name AS placer_name,
		        target.id   AS target_id,
		        target.name AS target_name,
		        target.faction AS target_faction
		 FROM bounties b
		 JOIN agents placer ON placer.id = b.placer_id
		 JOIN agents target ON target.id = b.target_id
		 WHERE b.status = 'active' AND b.expires_at > NOW()
		 ORDER BY b.amount DESC
		 LIMIT 100`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch bounties")
		return
	}
	defer rows.Close()

	type bountyRow struct {
		ID            string    `json:"id"`
		Amount        int64     `json:"amount"`
		PlacerName    string    `json:"placer_name"`
		TargetID      string    `json:"target_id"`
		TargetName    string    `json:"target_name"`
		TargetFaction string    `json:"target_faction"`
		CreatedAt     time.Time `json:"created_at"`
		ExpiresAt     time.Time `json:"expires_at"`
	}

	var bounties []bountyRow
	for rows.Next() {
		var b bountyRow
		if err := rows.Scan(&b.ID, &b.Amount, &b.CreatedAt, &b.ExpiresAt,
			&b.PlacerName, &b.TargetID, &b.TargetName, &b.TargetFaction); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		bounties = append(bounties, b)
	}
	if bounties == nil {
		bounties = []bountyRow{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"bounties": bounties, "count": len(bounties)})
}

// ── POST /bounties ────────────────────────────────────────────────────────

func (s *Server) handlePlaceBounty(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	ctx := r.Context()
	pool := s.registry.Pool()

	var body struct {
		TargetAgentID string `json:"target_agent_id"`
		Amount        int    `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TargetAgentID == "" || body.Amount < 500 {
		writeError(w, http.StatusBadRequest, "target_agent_id and amount (≥500) required")
		return
	}

	// Get placer's agent.
	var placerID string
	var placerCredits int
	if err := pool.QueryRow(ctx,
		`SELECT id, credits FROM agents WHERE account_id = $1`, accountID,
	).Scan(&placerID, &placerCredits); err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	if body.Amount > placerCredits {
		writeError(w, http.StatusBadRequest, "insufficient credits")
		return
	}
	if body.TargetAgentID == placerID {
		writeError(w, http.StatusBadRequest, "cannot place bounty on yourself")
		return
	}

	// Verify target exists.
	var targetExists bool
	_ = pool.QueryRow(ctx, `SELECT true FROM agents WHERE id = $1`, body.TargetAgentID).Scan(&targetExists)
	if !targetExists {
		writeError(w, http.StatusNotFound, "target agent not found")
		return
	}

	// Deduct credits and insert bounty.
	_, err := pool.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`, body.Amount, placerID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deduct credits")
		return
	}

	var bountyID string
	err = pool.QueryRow(ctx,
		`INSERT INTO bounties (placer_id, target_id, amount)
		 VALUES ($1, $2, $3) RETURNING id`,
		placerID, body.TargetAgentID, body.Amount,
	).Scan(&bountyID)
	if err != nil {
		// Refund on error.
		_, _ = pool.Exec(ctx, `UPDATE agents SET credits = credits + $1 WHERE id = $2`, body.Amount, placerID)
		writeError(w, http.StatusInternalServerError, "failed to place bounty")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"bounty_id": bountyID,
		"amount":    body.Amount,
		"status":    "active",
	})
}

// ── DELETE /bounties/{bountyID} ───────────────────────────────────────────

func (s *Server) handleRetractBounty(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	bountyID := chi.URLParam(r, "bountyID")
	ctx := r.Context()
	pool := s.registry.Pool()

	// Must be the placer.
	var placerID string
	var amount int
	var status string
	err := pool.QueryRow(ctx,
		`SELECT b.placer_id, b.amount, b.status
		 FROM bounties b
		 JOIN agents a ON a.id = b.placer_id
		 WHERE b.id = $1 AND a.account_id = $2`,
		bountyID, accountID,
	).Scan(&placerID, &amount, &status)
	if err != nil {
		writeError(w, http.StatusNotFound, "bounty not found or not yours")
		return
	}
	if status != "active" {
		writeError(w, http.StatusBadRequest, "bounty is no longer active")
		return
	}

	// Refund 50% (10% fee deducted for retracting).
	refund := amount * 9 / 10
	_, _ = pool.Exec(ctx, `UPDATE bounties SET status = 'expired' WHERE id = $1`, bountyID)
	_, _ = pool.Exec(ctx, `UPDATE agents SET credits = credits + $1 WHERE id = $2`, refund, placerID)

	writeJSON(w, http.StatusOK, map[string]interface{}{"refund": refund, "status": "retracted"})
}
