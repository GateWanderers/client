package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleMarketPosts returns all trading posts with location info.
// GET /market/posts (public)
func (s *Server) handleMarketPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.registry.Pool().Query(r.Context(),
		`SELECT p.name, p.system_name, p.galaxy_id, tp.resource_type, tp.current_price, tp.supply
		 FROM trading_posts tp
		 JOIN planets p ON p.id = tp.planet_id
		 ORDER BY p.name, tp.resource_type`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch trading posts")
		return
	}
	defer rows.Close()

	type tradingPostItem struct {
		PlanetName   string `json:"planet_name"`
		SystemName   string `json:"system_name"`
		GalaxyID     string `json:"galaxy_id"`
		ResourceType string `json:"resource_type"`
		CurrentPrice int    `json:"current_price"`
		Supply       int    `json:"supply"`
	}

	posts := []tradingPostItem{}
	for rows.Next() {
		var item tradingPostItem
		if err := rows.Scan(
			&item.PlanetName,
			&item.SystemName,
			&item.GalaxyID,
			&item.ResourceType,
			&item.CurrentPrice,
			&item.Supply,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan trading post row")
			return
		}
		posts = append(posts, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "trading post rows error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"posts": posts})
}

// handleCreateTradeOffer creates a market order for the authenticated agent.
// POST /market/trade (auth required)
func (s *Server) handleCreateTradeOffer(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		ResourceType string `json:"resource_type"`
		Quantity     int    `json:"quantity"`
		PricePerUnit int    `json:"price_per_unit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ResourceType == "" || req.Quantity <= 0 || req.PricePerUnit <= 0 {
		writeError(w, http.StatusBadRequest, "resource_type, quantity (>0), and price_per_unit (>0) are required")
		return
	}

	// Resolve agent_id from account.
	var agentID string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT id FROM agents WHERE account_id = $1`,
		accountID,
	).Scan(&agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Insert market order.
	var orderID string
	err = s.registry.Pool().QueryRow(r.Context(),
		`INSERT INTO market_orders (seller_id, resource_type, quantity, price_per_unit)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		agentID, req.ResourceType, req.Quantity, req.PricePerUnit,
	).Scan(&orderID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create trade offer")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"order_id": orderID})
}

// handleMarketOrders returns all open market orders.
// GET /market/orders (auth required)
func (s *Server) handleMarketOrders(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := s.registry.Pool().Query(r.Context(),
		`SELECT id, seller_id, resource_type, quantity, price_per_unit, created_at
		 FROM market_orders
		 WHERE status = 'open'
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch market orders")
		return
	}
	defer rows.Close()

	type orderItem struct {
		ID           string    `json:"id"`
		SellerID     string    `json:"seller_id"`
		ResourceType string    `json:"resource_type"`
		Quantity     int       `json:"quantity"`
		PricePerUnit int       `json:"price_per_unit"`
		CreatedAt    time.Time `json:"created_at"`
	}

	orders := []orderItem{}
	for rows.Next() {
		var item orderItem
		if err := rows.Scan(
			&item.ID,
			&item.SellerID,
			&item.ResourceType,
			&item.Quantity,
			&item.PricePerUnit,
			&item.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan order row")
			return
		}
		orders = append(orders, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "market order rows error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"orders": orders})
}

// handleAgentInventory returns the authenticated agent's current inventory.
// GET /agent/inventory (auth required)
func (s *Server) handleAgentInventory(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var agentID string
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT id FROM agents WHERE account_id = $1`,
		accountID,
	).Scan(&agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	rows, err := s.registry.Pool().Query(r.Context(),
		`SELECT resource_type, quantity FROM inventories WHERE agent_id = $1 ORDER BY resource_type`,
		agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch inventory")
		return
	}
	defer rows.Close()

	type inventoryItem struct {
		ResourceType string `json:"resource_type"`
		Quantity     int    `json:"quantity"`
	}

	inventory := []inventoryItem{}
	for rows.Next() {
		var item inventoryItem
		if err := rows.Scan(&item.ResourceType, &item.Quantity); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan inventory row")
			return
		}
		inventory = append(inventory, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "inventory rows error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"inventory": inventory})
}
