package ticker

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// processExpiredAuctions schließt alle Auktionen ab, deren expires_at_tick <= tickNumber.
// - Hat die Auktion Gebote: Credits an Verkäufer, Ressource an Höchstbieter
// - Hat keine Gebote: Ressource zurück an Verkäufer, status='expired'
func processExpiredAuctions(ctx context.Context, pool *pgxpool.Pool, tickNumber int64) {
	rows, err := pool.Query(ctx,
		`SELECT id, seller_agent_id, resource_type, quantity, current_bid, current_bidder_id
		 FROM auctions
		 WHERE status = 'active' AND expires_at_tick <= $1`,
		tickNumber,
	)
	if err != nil {
		slog.Error("processExpiredAuctions: query failed", "err", err)
		return
	}

	type expiredAuction struct {
		ID              string
		SellerAgentID   string
		ResourceType    string
		Quantity        int64
		CurrentBid      *int64
		CurrentBidderID *string
	}

	var expired []expiredAuction
	for rows.Next() {
		var a expiredAuction
		if err := rows.Scan(&a.ID, &a.SellerAgentID, &a.ResourceType, &a.Quantity,
			&a.CurrentBid, &a.CurrentBidderID); err == nil {
			expired = append(expired, a)
		}
	}
	rows.Close()

	for _, a := range expired {
		tx, err := pool.Begin(ctx)
		if err != nil {
			slog.Error("processExpiredAuctions: begin tx failed", "auction", a.ID, "err", err)
			continue
		}

		if a.CurrentBidderID != nil && *a.CurrentBidderID != "" {
			// Auction has a winner: give resource to highest bidder, credits to seller.
			_, err = tx.Exec(ctx,
				`INSERT INTO inventories (agent_id, resource_type, quantity)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (agent_id, resource_type)
				 DO UPDATE SET quantity = inventories.quantity + excluded.quantity`,
				*a.CurrentBidderID, a.ResourceType, a.Quantity,
			)
			if err != nil {
				slog.Error("processExpiredAuctions: give resource to bidder failed",
					"auction", a.ID, "err", err)
				tx.Rollback(ctx) //nolint:errcheck
				continue
			}

			_, err = tx.Exec(ctx,
				`UPDATE agents SET credits = credits + $1 WHERE id = $2`,
				*a.CurrentBid, a.SellerAgentID,
			)
			if err != nil {
				slog.Error("processExpiredAuctions: pay seller failed",
					"auction", a.ID, "err", err)
				tx.Rollback(ctx) //nolint:errcheck
				continue
			}

			_, err = tx.Exec(ctx,
				`UPDATE auctions SET status = 'sold' WHERE id = $1`,
				a.ID,
			)
			if err != nil {
				slog.Error("processExpiredAuctions: mark sold failed",
					"auction", a.ID, "err", err)
				tx.Rollback(ctx) //nolint:errcheck
				continue
			}

			if err := tx.Commit(ctx); err != nil {
				slog.Error("processExpiredAuctions: commit failed", "auction", a.ID, "err", err)
				continue
			}
			slog.Info("processExpiredAuctions: auction sold",
				"auction", a.ID,
				"seller", a.SellerAgentID,
				"buyer", *a.CurrentBidderID,
				"bid", *a.CurrentBid,
				"resource", a.ResourceType,
				"quantity", a.Quantity,
			)
		} else {
			// No bids: return resource to seller, mark expired.
			_, err = tx.Exec(ctx,
				`INSERT INTO inventories (agent_id, resource_type, quantity)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (agent_id, resource_type)
				 DO UPDATE SET quantity = inventories.quantity + excluded.quantity`,
				a.SellerAgentID, a.ResourceType, a.Quantity,
			)
			if err != nil {
				slog.Error("processExpiredAuctions: return resource to seller failed",
					"auction", a.ID, "err", err)
				tx.Rollback(ctx) //nolint:errcheck
				continue
			}

			_, err = tx.Exec(ctx,
				`UPDATE auctions SET status = 'expired' WHERE id = $1`,
				a.ID,
			)
			if err != nil {
				slog.Error("processExpiredAuctions: mark expired failed",
					"auction", a.ID, "err", err)
				tx.Rollback(ctx) //nolint:errcheck
				continue
			}

			if err := tx.Commit(ctx); err != nil {
				slog.Error("processExpiredAuctions: commit failed", "auction", a.ID, "err", err)
				continue
			}
			slog.Info("processExpiredAuctions: auction expired (no bids)",
				"auction", a.ID,
				"seller", a.SellerAgentID,
				"resource", a.ResourceType,
				"quantity", a.Quantity,
			)
		}
	}
}
