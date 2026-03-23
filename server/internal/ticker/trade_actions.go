package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BuyResult holds the outcome of a BUY action.
type BuyResult struct {
	ResourceType string
	Amount       string
	Cost         int
	PayloadEN    string
	PayloadDE    string
}

// SellResult holds the outcome of a SELL action.
type SellResult struct {
	ResourceType string
	Amount       string
	Gain         int
	PayloadEN    string
	PayloadDE    string
}

// TradeResult holds the outcome of an ACCEPT_TRADE action.
type TradeResult struct {
	PayloadEN string
	PayloadDE string
	Success   bool
}

type buyParams struct {
	ResourceType string `json:"resource_type"`
	Quantity     int    `json:"quantity"`
}

type sellParams struct {
	ResourceType string `json:"resource_type"`
	Quantity     int    `json:"quantity"`
}

type acceptTradeParams struct {
	OrderID string `json:"order_id"`
}

// getAgentPlanetID returns the planet_id (as *string) for the agent's ship.
func getAgentPlanetID(ctx context.Context, pool *pgxpool.Pool, agentID string) (*string, error) {
	var planetID *string
	err := pool.QueryRow(ctx,
		`SELECT planet_id FROM ships WHERE agent_id = $1 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&planetID)
	return planetID, err
}

// getPlanetName returns the planet's name by id.
func getPlanetName(ctx context.Context, pool *pgxpool.Pool, planetID string) string {
	var name string
	_ = pool.QueryRow(ctx, `SELECT name FROM planets WHERE id = $1`, planetID).Scan(&name)
	if name == "" {
		return planetID
	}
	return name
}

// processBuy handles the BUY action.
// Parameters: {"resource_type": "naquadah", "quantity": 10}
func processBuy(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) BuyResult {
	var p buyParams
	if err := json.Unmarshal(params, &p); err != nil || p.ResourceType == "" || p.Quantity <= 0 {
		return BuyResult{
			PayloadEN: "Buy failed: invalid parameters.",
			PayloadDE: "Kauf fehlgeschlagen: ungültige Parameter.",
		}
	}

	// 1. Get agent ship planet_id.
	planetID, err := getAgentPlanetID(ctx, pool, agentID)
	if err != nil || planetID == nil {
		return BuyResult{
			PayloadEN: "Buy failed: not landed on a planet.",
			PayloadDE: "Kauf fehlgeschlagen: nicht auf einem Planeten gelandet.",
		}
	}

	// 2. Find trading post.
	var postID string
	var currentPrice, supply int
	err = pool.QueryRow(ctx,
		`SELECT id, current_price, supply FROM trading_posts WHERE planet_id = $1 AND resource_type = $2`,
		*planetID, p.ResourceType,
	).Scan(&postID, &currentPrice, &supply)
	if err != nil {
		return BuyResult{
			PayloadEN: fmt.Sprintf("No trading post for %s at this location.", p.ResourceType),
			PayloadDE: fmt.Sprintf("Kein Handelsposten für %s an diesem Ort.", p.ResourceType),
		}
	}

	totalCost := currentPrice * p.Quantity

	// 3. Check agent's credits.
	var credits int
	err = pool.QueryRow(ctx,
		`SELECT credits FROM agents WHERE id = $1`,
		agentID,
	).Scan(&credits)
	if err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: could not read agent credits.",
			PayloadDE: "Kauf fehlgeschlagen: Credits konnten nicht gelesen werden.",
		}
	}
	if credits < totalCost {
		return BuyResult{
			PayloadEN: fmt.Sprintf("Insufficient credits (need %d, have %d).", totalCost, credits),
			PayloadDE: fmt.Sprintf("Unzureichende Credits (benötigt %d, vorhanden %d).", totalCost, credits),
		}
	}

	// 4. Execute transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: transaction error.",
			PayloadDE: "Kauf fehlgeschlagen: Transaktionsfehler.",
		}
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`,
		totalCost, agentID,
	)
	if err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: could not deduct credits.",
			PayloadDE: "Kauf fehlgeschlagen: Credits konnten nicht abgezogen werden.",
		}
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type) DO UPDATE
		   SET quantity = inventories.quantity + EXCLUDED.quantity`,
		agentID, p.ResourceType, p.Quantity,
	)
	if err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: could not update inventory.",
			PayloadDE: "Kauf fehlgeschlagen: Inventar konnte nicht aktualisiert werden.",
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE trading_posts SET supply = supply - $1, demand = demand + $1 WHERE id = $2`,
		p.Quantity, postID,
	)
	if err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: could not update trading post.",
			PayloadDE: "Kauf fehlgeschlagen: Handelsposten konnte nicht aktualisiert werden.",
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return BuyResult{
			PayloadEN: "Buy failed: transaction commit error.",
			PayloadDE: "Kauf fehlgeschlagen: Transaktions-Commit-Fehler.",
		}
	}

	planetName := getPlanetName(ctx, pool, *planetID)

	return BuyResult{
		ResourceType: p.ResourceType,
		Amount:       fmt.Sprintf("%d", p.Quantity),
		Cost:         totalCost,
		PayloadEN:    fmt.Sprintf("Purchased %d units of %s for %d credits at %s.", p.Quantity, p.ResourceType, totalCost, planetName),
		PayloadDE:    fmt.Sprintf("%d Einheiten %s für %d Credits bei %s gekauft.", p.Quantity, p.ResourceType, totalCost, planetName),
	}
}

// processSell handles the SELL action.
// Parameters: {"resource_type": "naquadah", "quantity": 10}
func processSell(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) SellResult {
	var p sellParams
	if err := json.Unmarshal(params, &p); err != nil || p.ResourceType == "" || p.Quantity <= 0 {
		return SellResult{
			PayloadEN: "Sell failed: invalid parameters.",
			PayloadDE: "Verkauf fehlgeschlagen: ungültige Parameter.",
		}
	}

	// 1. Get agent ship planet_id.
	planetID, err := getAgentPlanetID(ctx, pool, agentID)
	if err != nil || planetID == nil {
		return SellResult{
			PayloadEN: "Sell failed: not landed on a planet.",
			PayloadDE: "Verkauf fehlgeschlagen: nicht auf einem Planeten gelandet.",
		}
	}

	// 2. Find trading post price.
	var postID string
	var currentPrice int
	err = pool.QueryRow(ctx,
		`SELECT id, current_price FROM trading_posts WHERE planet_id = $1 AND resource_type = $2`,
		*planetID, p.ResourceType,
	).Scan(&postID, &currentPrice)
	if err != nil {
		return SellResult{
			PayloadEN: fmt.Sprintf("No trading post for %s at this location.", p.ResourceType),
			PayloadDE: fmt.Sprintf("Kein Handelsposten für %s an diesem Ort.", p.ResourceType),
		}
	}

	// 3. Check inventory.
	var invQty int
	err = pool.QueryRow(ctx,
		`SELECT quantity FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
		agentID, p.ResourceType,
	).Scan(&invQty)
	if err != nil || invQty < p.Quantity {
		return SellResult{
			PayloadEN: fmt.Sprintf("Insufficient %s in inventory.", p.ResourceType),
			PayloadDE: fmt.Sprintf("Unzureichend %s im Inventar.", p.ResourceType),
		}
	}

	totalGain := currentPrice * p.Quantity

	// 4. Execute transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SellResult{
			PayloadEN: "Sell failed: transaction error.",
			PayloadDE: "Verkauf fehlgeschlagen: Transaktionsfehler.",
		}
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Update or delete inventory row.
	remaining := invQty - p.Quantity
	if remaining == 0 {
		_, err = tx.Exec(ctx,
			`DELETE FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
			agentID, p.ResourceType,
		)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE inventories SET quantity = quantity - $1 WHERE agent_id = $2 AND resource_type = $3`,
			p.Quantity, agentID, p.ResourceType,
		)
	}
	if err != nil {
		return SellResult{
			PayloadEN: "Sell failed: could not update inventory.",
			PayloadDE: "Verkauf fehlgeschlagen: Inventar konnte nicht aktualisiert werden.",
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
		totalGain, agentID,
	)
	if err != nil {
		return SellResult{
			PayloadEN: "Sell failed: could not update credits.",
			PayloadDE: "Verkauf fehlgeschlagen: Credits konnten nicht aktualisiert werden.",
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE trading_posts SET supply = supply + $1, demand = GREATEST(demand - $1, 0) WHERE id = $2`,
		p.Quantity, postID,
	)
	if err != nil {
		return SellResult{
			PayloadEN: "Sell failed: could not update trading post.",
			PayloadDE: "Verkauf fehlgeschlagen: Handelsposten konnte nicht aktualisiert werden.",
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return SellResult{
			PayloadEN: "Sell failed: transaction commit error.",
			PayloadDE: "Verkauf fehlgeschlagen: Transaktions-Commit-Fehler.",
		}
	}

	planetName := getPlanetName(ctx, pool, *planetID)

	return SellResult{
		ResourceType: p.ResourceType,
		Amount:       fmt.Sprintf("%d", p.Quantity),
		Gain:         totalGain,
		PayloadEN:    fmt.Sprintf("Sold %d units of %s for %d credits at %s.", p.Quantity, p.ResourceType, totalGain, planetName),
		PayloadDE:    fmt.Sprintf("%d Einheiten %s für %d Credits bei %s verkauft.", p.Quantity, p.ResourceType, totalGain, planetName),
	}
}

// processAcceptTrade handles the ACCEPT_TRADE action.
// Parameters: {"order_id": "uuid"}
func processAcceptTrade(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) TradeResult {
	var p acceptTradeParams
	if err := json.Unmarshal(params, &p); err != nil || p.OrderID == "" {
		return TradeResult{
			PayloadEN: "Accept trade failed: invalid parameters.",
			PayloadDE: "Handelsannahme fehlgeschlagen: ungültige Parameter.",
		}
	}

	// 1. Load the market order.
	var orderID, sellerID, resourceType, status string
	var quantity, pricePerUnit int
	err := pool.QueryRow(ctx,
		`SELECT id, seller_id, resource_type, quantity, price_per_unit, status
		 FROM market_orders WHERE id = $1`,
		p.OrderID,
	).Scan(&orderID, &sellerID, &resourceType, &quantity, &pricePerUnit, &status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TradeResult{
				PayloadEN: "Trade order not found.",
				PayloadDE: "Handelsauftrag nicht gefunden.",
			}
		}
		return TradeResult{
			PayloadEN: "Accept trade failed: database error.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Datenbankfehler.",
		}
	}

	if status != "open" {
		return TradeResult{
			PayloadEN: "Trade order is no longer open.",
			PayloadDE: "Handelsauftrag ist nicht mehr offen.",
		}
	}

	// 2. Cannot accept own trade.
	if sellerID == agentID {
		return TradeResult{
			PayloadEN: "Cannot accept your own trade offer.",
			PayloadDE: "Du kannst dein eigenes Handelsangebot nicht annehmen.",
		}
	}

	totalCost := quantity * pricePerUnit

	// 3. Execute transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: transaction error.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Transaktionsfehler.",
		}
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Check buyer credits.
	var buyerCredits int
	if err := tx.QueryRow(ctx,
		`SELECT credits FROM agents WHERE id = $1`,
		agentID,
	).Scan(&buyerCredits); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: could not read buyer credits.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Käufer-Credits konnten nicht gelesen werden.",
		}
	}
	if buyerCredits < totalCost {
		return TradeResult{
			PayloadEN: fmt.Sprintf("Insufficient credits (need %d, have %d).", totalCost, buyerCredits),
			PayloadDE: fmt.Sprintf("Unzureichende Credits (benötigt %d, vorhanden %d).", totalCost, buyerCredits),
		}
	}

	// Check seller inventory.
	var sellerQty int
	err = tx.QueryRow(ctx,
		`SELECT quantity FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
		sellerID, resourceType,
	).Scan(&sellerQty)
	if err != nil || sellerQty < quantity {
		return TradeResult{
			PayloadEN: fmt.Sprintf("Seller has insufficient %s in inventory.", resourceType),
			PayloadDE: fmt.Sprintf("Verkäufer hat unzureichend %s im Inventar.", resourceType),
		}
	}

	// Transfer credits.
	if _, err := tx.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`,
		totalCost, agentID,
	); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: could not deduct buyer credits.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Käufer-Credits konnten nicht abgezogen werden.",
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
		totalCost, sellerID,
	); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: could not add seller credits.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Verkäufer-Credits konnten nicht hinzugefügt werden.",
		}
	}

	// Transfer inventory: remove from seller.
	sellerRemaining := sellerQty - quantity
	if sellerRemaining == 0 {
		if _, err := tx.Exec(ctx,
			`DELETE FROM inventories WHERE agent_id = $1 AND resource_type = $2`,
			sellerID, resourceType,
		); err != nil {
			return TradeResult{
				PayloadEN: "Accept trade failed: could not update seller inventory.",
				PayloadDE: "Handelsannahme fehlgeschlagen: Inventar des Verkäufers konnte nicht aktualisiert werden.",
			}
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE inventories SET quantity = quantity - $1 WHERE agent_id = $2 AND resource_type = $3`,
			quantity, sellerID, resourceType,
		); err != nil {
			return TradeResult{
				PayloadEN: "Accept trade failed: could not update seller inventory.",
				PayloadDE: "Handelsannahme fehlgeschlagen: Inventar des Verkäufers konnte nicht aktualisiert werden.",
			}
		}
	}

	// Add to buyer inventory.
	if _, err := tx.Exec(ctx,
		`INSERT INTO inventories (agent_id, resource_type, quantity)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (agent_id, resource_type) DO UPDATE
		   SET quantity = inventories.quantity + EXCLUDED.quantity`,
		agentID, resourceType, quantity,
	); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: could not update buyer inventory.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Inventar des Käufers konnte nicht aktualisiert werden.",
		}
	}

	// Mark order as accepted.
	if _, err := tx.Exec(ctx,
		`UPDATE market_orders SET status = 'accepted', buyer_id = $1 WHERE id = $2`,
		agentID, orderID,
	); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: could not update order status.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Auftragsstatus konnte nicht aktualisiert werden.",
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return TradeResult{
			PayloadEN: "Accept trade failed: transaction commit error.",
			PayloadDE: "Handelsannahme fehlgeschlagen: Transaktions-Commit-Fehler.",
		}
	}

	return TradeResult{
		Success:   true,
		PayloadEN: fmt.Sprintf("Trade accepted. You acquired %d units of %s for %d credits.", quantity, resourceType, totalCost),
		PayloadDE: fmt.Sprintf("Handel angenommen. Du hast %d Einheiten %s für %d Credits erworben.", quantity, resourceType, totalCost),
	}
}
