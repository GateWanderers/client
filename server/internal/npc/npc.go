package npc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Seeder seeds NPC faction data into the database.
type Seeder struct{ pool *pgxpool.Pool }

// NewSeeder creates a new Seeder backed by the given pool.
func NewSeeder(pool *pgxpool.Pool) *Seeder { return &Seeder{pool: pool} }

// factionSeed holds one NPC faction's seed data.
type factionSeed struct {
	ID               string
	Name             string
	NameDE           string
	GalaxyID         string
	FleetStrength    int
	Agenda           string
	TerritorySystems []string
}

// Seed inserts NPC factions if none exist.
func (s *Seeder) Seed(ctx context.Context) error {
	factions := []factionSeed{
		{
			ID:               "goa_uld_remnants",
			Name:             "Goa'uld Remnants",
			NameDE:           "Goa'uld-Überreste",
			GalaxyID:         "milky_way",
			FleetStrength:    150,
			Agenda:           "expansionist",
			TerritorySystems: []string{"netu", "dakara"},
		},
		{
			ID:               "jaffa_patrol",
			Name:             "Jaffa Patrol",
			NameDE:           "Jaffa-Patrouille",
			GalaxyID:         "milky_way",
			FleetStrength:    80,
			Agenda:           "defensive",
			TerritorySystems: []string{"chulak", "p3x_888"},
		},
		{
			ID:               "lucian_alliance",
			Name:             "Lucian Alliance",
			NameDE:           "Lucianische Allianz",
			GalaxyID:         "milky_way",
			FleetStrength:    100,
			Agenda:           "aggressive",
			TerritorySystems: []string{"hebridan", "vyus"},
		},
		{
			ID:               "wraith",
			Name:             "Wraith",
			NameDE:           "Wraith",
			GalaxyID:         "pegasus",
			FleetStrength:    200,
			Agenda:           "aggressive",
			TerritorySystems: []string{"sateda", "hoff", "doranda"},
		},
		{
			ID:               "replicators",
			Name:             "Replicators",
			NameDE:           "Replikatoren",
			GalaxyID:         "milky_way",
			FleetStrength:    120,
			Agenda:           "expansionist",
			TerritorySystems: []string{},
		},
		{
			ID:               "ori_prior",
			Name:             "Ori Prior Crusade",
			NameDE:           "Ori-Prior-Kreuzzug",
			GalaxyID:         "milky_way",
			FleetStrength:    180,
			Agenda:           "expansionist",
			TerritorySystems: []string{"camelot"},
		},
		{
			ID:               "ancient_construct",
			Name:             "Ancient Construct Defense",
			NameDE:           "Altvorderen-Konstrukt-Verteidigung",
			GalaxyID:         "destiny",
			FleetStrength:    160,
			Agenda:           "defensive",
			TerritorySystems: []string{"desert_ruins", "unnamed_outpost"},
		},
	}

	for _, f := range factions {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO npc_factions (id, name, name_de, galaxy_id, fleet_strength, agenda, territory_systems)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			f.ID, f.Name, f.NameDE, f.GalaxyID, f.FleetStrength, f.Agenda, f.TerritorySystems,
		)
		if err != nil {
			return fmt.Errorf("insert npc faction %s: %w", f.ID, err)
		}
	}

	slog.Info("npc factions seeded")
	return nil
}
