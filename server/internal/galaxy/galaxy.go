package galaxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Seeder seeds the planets table if it is empty.
type Seeder struct {
	pool *pgxpool.Pool
}

// NewSeeder creates a new Seeder backed by the given pool.
func NewSeeder(pool *pgxpool.Pool) *Seeder { return &Seeder{pool: pool} }

// Seed inserts all galaxy data if the planets table is empty.
func (s *Seeder) Seed(ctx context.Context) error {
	// Check if already seeded.
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM planets`).Scan(&count); err != nil {
		return fmt.Errorf("count planets: %w", err)
	}
	if count > 0 {
		return nil
	}

	total := 0
	for galaxyID, systems := range galaxies() {
		for _, sys := range systems {
			for _, planet := range sys.Planets {
				resourcesJSON, err := json.Marshal(planet.Resources)
				if err != nil {
					return fmt.Errorf("marshal resources for %s: %w", planet.Name, err)
				}

				npcs := planet.NPCs
				if npcs == nil {
					npcs = []NPCSeed{}
				}
				npcJSON, err := json.Marshal(npcs)
				if err != nil {
					return fmt.Errorf("marshal npcs for %s: %w", planet.Name, err)
				}

				_, err = s.pool.Exec(ctx,
					`INSERT INTO planets
					 (galaxy_id, system_id, system_name, system_x, system_y, name, gate_address, resource_nodes, npc_presence)
					 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
					galaxyID, sys.ID, sys.Name, sys.X, sys.Y,
					planet.Name, planet.GateAddress, resourcesJSON, npcJSON,
				)
				if err != nil {
					return fmt.Errorf("insert planet %s: %w", planet.Name, err)
				}
				total++
			}
		}
	}

	slog.Info("galaxy seeded", "planets", total)
	return nil
}

// SeedSystemControl populates system_control from planets + npc_factions data.
// Idempotent — skips if already seeded. Must be called after Seed() and NPC seeder.
func (s *Seeder) SeedSystemControl(ctx context.Context) error {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM system_control`).Scan(&count); err != nil {
		return fmt.Errorf("count system_control: %w", err)
	}
	if count > 0 {
		return nil
	}

	// Fetch all distinct systems from planets.
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT system_id, galaxy_id FROM planets ORDER BY galaxy_id, system_id`,
	)
	if err != nil {
		return fmt.Errorf("query systems: %w", err)
	}
	type systemKey struct{ SystemID, GalaxyID string }
	var systems []systemKey
	for rows.Next() {
		var k systemKey
		if err := rows.Scan(&k.SystemID, &k.GalaxyID); err != nil {
			rows.Close()
			return fmt.Errorf("scan system: %w", err)
		}
		systems = append(systems, k)
	}
	rows.Close()

	// Build a map: system_id → (npc_faction_id, strength) from npc_factions.territory_systems.
	type npcControl struct {
		FactionID string
		Strength  int
	}
	npcRows, err := s.pool.Query(ctx,
		`SELECT id, fleet_strength, territory_systems FROM npc_factions`,
	)
	if err != nil {
		return fmt.Errorf("query npc_factions: %w", err)
	}
	systemNPC := map[string]npcControl{}
	for npcRows.Next() {
		var factionID string
		var strength int
		var territorySystems []string
		if err := npcRows.Scan(&factionID, &strength, &territorySystems); err != nil {
			npcRows.Close()
			return fmt.Errorf("scan npc faction: %w", err)
		}
		for _, sysID := range territorySystems {
			if _, exists := systemNPC[sysID]; !exists {
				systemNPC[sysID] = npcControl{FactionID: factionID, Strength: strength}
			}
		}
	}
	npcRows.Close()

	// Insert one row per system.
	inserted := 0
	for _, sys := range systems {
		// Income based on number of resource nodes in the system.
		var resourceCount int
		_ = s.pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(jsonb_array_length(resource_nodes)), 0)
			 FROM planets WHERE system_id = $1 AND galaxy_id = $2`,
			sys.SystemID, sys.GalaxyID,
		).Scan(&resourceCount)
		incomePer := 50 + resourceCount*25

		ctrl, hasNPC := systemNPC[sys.SystemID]
		controllerFaction := "unclaimed"
		controllerType := "unclaimed"
		defenseStrength := 0
		if hasNPC {
			controllerFaction = ctrl.FactionID
			controllerType = "npc"
			defenseStrength = ctrl.Strength
		}

		_, err := s.pool.Exec(ctx,
			`INSERT INTO system_control
			 (system_id, galaxy_id, controller_faction, controller_type, defense_strength, income_per_tick)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			sys.SystemID, sys.GalaxyID, controllerFaction, controllerType, defenseStrength, incomePer,
		)
		if err != nil {
			return fmt.Errorf("insert system_control %s: %w", sys.SystemID, err)
		}
		inserted++
	}

	slog.Info("system_control seeded", "systems", inserted)
	return nil
}

// galaxies returns the full seed data for all three galaxies.
func galaxies() map[string][]SystemSeed {
	return map[string][]SystemSeed{
		"milky_way": milkyWaySystems(),
		"pegasus":   pegasusSystems(),
		"destiny":   destinySystems(),
	}
}
