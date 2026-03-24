package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// auctionRow represents a single auction record returned to clients.
type auctionRow struct {
	ID              string    `json:"id"`
	SellerAgentID   string    `json:"seller_agent_id"`
	SellerName      string    `json:"seller_name"`
	ResourceType    string    `json:"resource_type"`
	Quantity        int64     `json:"quantity"`
	StartingPrice   int64     `json:"starting_price"`
	BuyoutPrice     *int64    `json:"buyout_price"`
	CurrentBid      *int64    `json:"current_bid"`
	CurrentBidderID *string   `json:"current_bidder_id"`
	SystemID        *string   `json:"system_id"`
	ExpiresAtTick   int64     `json:"expires_at_tick"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// handleCreateAuction creates a new auction for the authenticated agent.
// POST /market/auction (auth required)
func (s *Server) handleCreateAuction(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var body struct {
		ResourceType  string `json:"resource_type"`
		Quantity      int64  `json:"quantity"`
		StartingPrice int64  `json:"starting_price"`
		BuyoutPrice   *int64 `json:"buyout_price"`
		DurationTicks int64  `json:"duration_ticks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.ResourceType == "" || body.Quantity <= 0 || body.StartingPrice <= 0 {
		writeError(w, http.StatusBadRequest, "resource_type, quantity (>0), and starting_price (>0) are required")
		return
	}
	if body.DurationTicks < 3 {
		body.DurationTicks = 3
	}
	if body.DurationTicks > 50 {
		body.DurationTicks = 50
	}

	// Get current tick number.
	var currentTick int64
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT tick_number FROM tick_state WHERE id = 1`,
	).Scan(&currentTick)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read tick state")
		return
	}
	expiresAtTick := currentTick + body.DurationTicks

	// Begin transaction: check inventory and create auction atomically.
	tx, err := s.registry.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Check and deduct from inventory.
	var currentQty int64
	err = tx.QueryRow(r.Context(),
		`SELECT quantity FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
		agentID, body.ResourceType,
	).Scan(&currentQty)
	if err != nil || currentQty < body.Quantity {
		writeError(w, http.StatusBadRequest, "insufficient inventory")
		return
	}

	_, err = tx.Exec(r.Context(),
		`UPDATE inventories SET quantity = quantity - $1
		 WHERE agent_id = $2 AND resource_type = $3`,
		body.Quantity, agentID, body.ResourceType,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deduct inventory")
		return
	}

	// Create the auction.
	var auctionID string
	err = tx.QueryRow(r.Context(),
		`INSERT INTO auctions
		   (seller_agent_id, resource_type, quantity, starting_price, buyout_price, expires_at_tick)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		agentID, body.ResourceType, body.Quantity, body.StartingPrice, body.BuyoutPrice, expiresAtTick,
	).Scan(&auctionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create auction")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}

	// Return the created auction.
	var auction auctionRow
	err = s.registry.Pool().QueryRow(r.Context(),
		`SELECT a.id, a.seller_agent_id, ag.name, a.resource_type, a.quantity,
		        a.starting_price, a.buyout_price, a.current_bid, a.current_bidder_id,
		        a.system_id, a.expires_at_tick, a.status, a.created_at
		 FROM auctions a
		 JOIN agents ag ON ag.id = a.seller_agent_id
		 WHERE a.id = $1`,
		auctionID,
	).Scan(
		&auction.ID, &auction.SellerAgentID, &auction.SellerName,
		&auction.ResourceType, &auction.Quantity, &auction.StartingPrice,
		&auction.BuyoutPrice, &auction.CurrentBid, &auction.CurrentBidderID,
		&auction.SystemID, &auction.ExpiresAtTick, &auction.Status, &auction.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch created auction")
		return
	}

	writeJSON(w, http.StatusCreated, auction)
}

// handleGetAuctions returns a list of auctions with optional filtering.
// GET /market/auctions (public)
func (s *Server) handleGetAuctions(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}

	query := `SELECT a.id, a.seller_agent_id, ag.name, a.resource_type, a.quantity,
		         a.starting_price, a.buyout_price, a.current_bid, a.current_bidder_id,
		         a.system_id, a.expires_at_tick, a.status, a.created_at
		  FROM auctions a
		  JOIN agents ag ON ag.id = a.seller_agent_id
		  WHERE a.status = $1`
	args := []any{status}

	if resourceType != "" {
		query += ` AND a.resource_type = $2`
		args = append(args, resourceType)
	}
	query += ` ORDER BY a.created_at DESC LIMIT 100`

	rows, err := s.registry.Pool().Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch auctions")
		return
	}
	defer rows.Close()

	auctions := []auctionRow{}
	for rows.Next() {
		var a auctionRow
		if err := rows.Scan(
			&a.ID, &a.SellerAgentID, &a.SellerName, &a.ResourceType, &a.Quantity,
			&a.StartingPrice, &a.BuyoutPrice, &a.CurrentBid, &a.CurrentBidderID,
			&a.SystemID, &a.ExpiresAtTick, &a.Status, &a.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan auction row")
			return
		}
		auctions = append(auctions, a)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "auction rows error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"auctions": auctions})
}

// handleBidAuction places a bid on an auction.
// POST /market/auction/{auctionID}/bid (auth required)
func (s *Server) handleBidAuction(w http.ResponseWriter, r *http.Request) {
	auctionID := chi.URLParam(r, "auctionID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var body struct {
		Amount int64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	tx, err := s.registry.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Fetch auction with lock.
	var auction struct {
		SellerAgentID   string
		StartingPrice   int64
		CurrentBid      *int64
		CurrentBidderID *string
		Status          string
	}
	err = tx.QueryRow(r.Context(),
		`SELECT seller_agent_id, starting_price, current_bid, current_bidder_id, status
		 FROM auctions WHERE id = $1 FOR UPDATE`,
		auctionID,
	).Scan(&auction.SellerAgentID, &auction.StartingPrice, &auction.CurrentBid,
		&auction.CurrentBidderID, &auction.Status)
	if err != nil {
		writeError(w, http.StatusNotFound, "auction not found")
		return
	}
	if auction.Status != "active" {
		writeError(w, http.StatusConflict, "auction is not active")
		return
	}
	if auction.SellerAgentID == agentID {
		writeError(w, http.StatusBadRequest, "cannot bid on your own auction")
		return
	}

	// Validate bid amount.
	minBid := auction.StartingPrice
	if auction.CurrentBid != nil {
		minBid = *auction.CurrentBid + 1
	}
	if body.Amount < minBid {
		writeError(w, http.StatusBadRequest, "bid amount too low")
		return
	}

	// Check bidder has enough credits.
	var bidderCredits int64
	err = tx.QueryRow(r.Context(),
		`SELECT credits FROM agents WHERE id = $1`,
		agentID,
	).Scan(&bidderCredits)
	if err != nil || bidderCredits < body.Amount {
		writeError(w, http.StatusBadRequest, "insufficient credits")
		return
	}

	// Deduct credits from new bidder.
	_, err = tx.Exec(r.Context(),
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`,
		body.Amount, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deduct credits from bidder")
		return
	}

	// Refund previous bidder if any.
	if auction.CurrentBidderID != nil && *auction.CurrentBidderID != "" {
		_, err = tx.Exec(r.Context(),
			`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
			*auction.CurrentBid, *auction.CurrentBidderID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to refund previous bidder")
			return
		}
	}

	// Update the auction.
	_, err = tx.Exec(r.Context(),
		`UPDATE auctions SET current_bid = $1, current_bidder_id = $2 WHERE id = $3`,
		body.Amount, agentID, auctionID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update auction")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "bid": body.Amount})
}

// handleBuyoutAuction performs an immediate buyout of an auction.
// POST /market/auction/{auctionID}/buyout (auth required)
func (s *Server) handleBuyoutAuction(w http.ResponseWriter, r *http.Request) {
	auctionID := chi.URLParam(r, "auctionID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	tx, err := s.registry.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Fetch auction with lock.
	var auction struct {
		SellerAgentID   string
		ResourceType    string
		Quantity        int64
		BuyoutPrice     *int64
		CurrentBid      *int64
		CurrentBidderID *string
		Status          string
	}
	err = tx.QueryRow(r.Context(),
		`SELECT seller_agent_id, resource_type, quantity, buyout_price,
		        current_bid, current_bidder_id, status
		 FROM auctions WHERE id = $1 FOR UPDATE`,
		auctionID,
	).Scan(&auction.SellerAgentID, &auction.ResourceType, &auction.Quantity,
		&auction.BuyoutPrice, &auction.CurrentBid, &auction.CurrentBidderID, &auction.Status)
	if err != nil {
		writeError(w, http.StatusNotFound, "auction not found")
		return
	}
	if auction.Status != "active" {
		writeError(w, http.StatusConflict, "auction is not active")
		return
	}
	if auction.BuyoutPrice == nil {
		writeError(w, http.StatusBadRequest, "auction has no buyout price")
		return
	}
	if auction.SellerAgentID == agentID {
		writeError(w, http.StatusBadRequest, "cannot buy out your own auction")
		return
	}

	buyoutPrice := *auction.BuyoutPrice

	// Check buyer has enough credits.
	var buyerCredits int64
	err = tx.QueryRow(r.Context(),
		`SELECT credits FROM agents WHERE id = $1`,
		agentID,
	).Scan(&buyerCredits)
	if err != nil || buyerCredits < buyoutPrice {
		writeError(w, http.StatusBadRequest, "insufficient credits")
		return
	}

	// Deduct credits from buyer.
	_, err = tx.Exec(r.Context(),
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`,
		buyoutPrice, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to deduct credits from buyer")
		return
	}

	// Refund previous bidder if any.
	if auction.CurrentBidderID != nil && *auction.CurrentBidderID != "" {
		_, err = tx.Exec(r.Context(),
			`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
			*auction.CurrentBid, *auction.CurrentBidderID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to refund previous bidder")
			return
		}
	}

	// Pay seller.
	_, err = tx.Exec(r.Context(),
		`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
		buyoutPrice, auction.SellerAgentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pay seller")
		return
	}

	// Give resource to buyer.
	_, err = tx.Exec(r.Context(),
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type)
		 DO UPDATE SET quantity = inventories.quantity + excluded.quantity`,
		agentID, auction.ResourceType, auction.Quantity,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to transfer resource to buyer")
		return
	}

	// Mark auction as sold.
	_, err = tx.Exec(r.Context(),
		`UPDATE auctions SET status = 'sold', current_bidder_id = $1, current_bid = $2 WHERE id = $3`,
		agentID, buyoutPrice, auctionID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update auction status")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "buyout_price": buyoutPrice})
}

// handleCancelAuction cancels an auction (seller only, only if no bids placed).
// DELETE /market/auction/{auctionID} (auth required)
func (s *Server) handleCancelAuction(w http.ResponseWriter, r *http.Request) {
	auctionID := chi.URLParam(r, "auctionID")
	accountID := accountIDFromContext(r.Context())
	agentID, ok := s.agentIDForAccount(r, accountID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	tx, err := s.registry.Pool().Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	// Fetch auction with lock.
	var auction struct {
		SellerAgentID string
		ResourceType  string
		Quantity      int64
		CurrentBid    *int64
		Status        string
	}
	err = tx.QueryRow(r.Context(),
		`SELECT seller_agent_id, resource_type, quantity, current_bid, status
		 FROM auctions WHERE id = $1 FOR UPDATE`,
		auctionID,
	).Scan(&auction.SellerAgentID, &auction.ResourceType, &auction.Quantity,
		&auction.CurrentBid, &auction.Status)
	if err != nil {
		writeError(w, http.StatusNotFound, "auction not found")
		return
	}
	if auction.SellerAgentID != agentID {
		writeError(w, http.StatusForbidden, "only the seller can cancel this auction")
		return
	}
	if auction.Status != "active" {
		writeError(w, http.StatusConflict, "auction is not active")
		return
	}
	if auction.CurrentBid != nil {
		writeError(w, http.StatusConflict, "cannot cancel auction with existing bids")
		return
	}

	// Return resource to seller.
	_, err = tx.Exec(r.Context(),
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type)
		 DO UPDATE SET quantity = inventories.quantity + excluded.quantity`,
		agentID, auction.ResourceType, auction.Quantity,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to return resource to seller")
		return
	}

	// Mark auction as cancelled.
	_, err = tx.Exec(r.Context(),
		`UPDATE auctions SET status = 'cancelled' WHERE id = $1`,
		auctionID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to cancel auction")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
