package economy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BasePrices defines the base price per resource type.
var BasePrices = map[string]int{
	"naquadah":    50,
	"trinium":     30,
	"naquadriah":  150,
	"ancient_tech": 200,
}

// planetResources maps planet names to the resources they sell.
var planetResources = []struct {
	Planet    string
	Resources []string
}{
	{"Terra Nova", []string{"naquadah", "trinium", "naquadriah", "ancient_tech"}},
	{"Abydos", []string{"naquadah"}},
	{"Chulak", []string{"trinium"}},
	{"Dakara", []string{"naquadah", "ancient_tech"}},
	{"Hebridan", []string{"trinium", "naquadah"}},
	{"Lantea", []string{"ancient_tech"}},
	{"Sateda", []string{"trinium"}},
	{"Genia", []string{"naquadriah"}},
	{"Novus", []string{"naquadah", "trinium"}},
}

// Seeder seeds economy data into the database.
type Seeder struct{ pool *pgxpool.Pool }

// NewSeeder creates a new Seeder backed by the given pool.
func NewSeeder(pool *pgxpool.Pool) *Seeder { return &Seeder{pool: pool} }

// Seed creates trading posts on specific planets if none exist yet.
func (s *Seeder) Seed(ctx context.Context) error {
	// Check if already seeded.
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM trading_posts`).Scan(&count); err != nil {
		return fmt.Errorf("count trading_posts: %w", err)
	}
	if count > 0 {
		return nil
	}

	total := 0
	for _, pr := range planetResources {
		var planetID string
		err := s.pool.QueryRow(ctx,
			`SELECT id FROM planets WHERE name = $1`,
			pr.Planet,
		).Scan(&planetID)
		if err != nil {
			// Planet not found — skip gracefully.
			slog.Warn("economy seeder: planet not found, skipping", "planet", pr.Planet)
			continue
		}

		for _, resource := range pr.Resources {
			basePrice := BasePrices[resource]
			_, err := s.pool.Exec(ctx,
				`INSERT INTO trading_posts (planet_id, resource_type, base_price, current_price)
				 VALUES ($1, $2, $3, $3) ON CONFLICT DO NOTHING`,
				planetID, resource, basePrice,
			)
			if err != nil {
				return fmt.Errorf("insert trading_post planet=%s resource=%s: %w", pr.Planet, resource, err)
			}
			total++
		}
	}

	slog.Info("economy seeded", "trading_posts", total)
	return nil
}
