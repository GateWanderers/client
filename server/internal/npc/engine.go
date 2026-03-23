package npc

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Engine runs NPC faction behavior each tick.
type Engine struct{ pool *pgxpool.Pool }

// NewEngine creates a new Engine backed by the given pool.
func NewEngine(pool *pgxpool.Pool) *Engine { return &Engine{pool: pool} }

// faction holds data for one NPC faction fetched from the database.
type faction struct {
	ID               string
	Name             string
	NameDE           string
	GalaxyID         string
	FleetStrength    int
	Agenda           string
	TerritorySystems []string
}

// RunTick executes all NPC faction behaviors for a given tick.
// Called by the Ticker at the end of each tick (after player actions, before price adjustment).
func (e *Engine) RunTick(ctx context.Context, tickNumber int64, rng *rand.Rand) {
	// 1. Fetch all factions.
	rows, err := e.pool.Query(ctx,
		`SELECT id, name, name_de, galaxy_id, fleet_strength, agenda, territory_systems FROM npc_factions`,
	)
	if err != nil {
		slog.Error("npc engine: fetch factions", "err", err)
		return
	}

	var factions []faction
	for rows.Next() {
		var f faction
		if err := rows.Scan(&f.ID, &f.Name, &f.NameDE, &f.GalaxyID, &f.FleetStrength, &f.Agenda, &f.TerritorySystems); err != nil {
			slog.Error("npc engine: scan faction", "err", err)
			rows.Close()
			return
		}
		factions = append(factions, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Error("npc engine: faction rows error", "err", err)
		return
	}

	// 2. Run behaviors for each faction.
	for _, f := range factions {
		e.runFactionBehavior(ctx, f, tickNumber, rng)
	}
}

// runFactionBehavior executes the per-faction tick logic.
func (e *Engine) runFactionBehavior(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	// --- ALL factions: 15% chance patrol event ---
	if rng.Float64() < 0.15 && len(f.TerritorySystems) > 0 {
		systemID := f.TerritorySystems[rng.Intn(len(f.TerritorySystems))]
		payloadEN := fmt.Sprintf("%s forces have been detected patrolling the %s system.", f.Name, systemID)
		payloadDE := fmt.Sprintf("%s-Kräfte wurden beim Patrouillieren im System %s entdeckt.", f.NameDE, systemID)
		e.insertWorldEvent(ctx, f.ID, "patrol", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)
	}

	// --- EXPANSIONIST factions: 10% chance expand territory ---
	if f.Agenda == "expansionist" && rng.Float64() < 0.10 {
		e.runExpansion(ctx, f, tickNumber, rng)
	}

	// --- AGGRESSIVE factions: 8% chance raid event ---
	if f.Agenda == "aggressive" && rng.Float64() < 0.08 {
		e.runRaid(ctx, f, tickNumber, rng)
	}

	// --- Special events by faction id ---
	switch f.ID {
	case "wraith":
		if rng.Float64() < 0.05 {
			e.runWraithCulling(ctx, f, tickNumber, rng)
		}
	case "replicators":
		if rng.Float64() < 0.03 {
			e.runReplicatorIncursion(ctx, f, tickNumber, rng)
		}
	}

	// --- ALL factions: 12% chance recapture a player-held system in their territory ---
	if rng.Float64() < 0.12 && len(f.TerritorySystems) > 0 {
		e.runRecapture(ctx, f, tickNumber, rng)
	}
}

// runExpansion handles the expansionist territory expansion behavior.
func (e *Engine) runExpansion(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	rows, err := e.pool.Query(ctx,
		`SELECT DISTINCT system_id FROM planets WHERE galaxy_id = $1
		 AND system_id != ALL(
		     SELECT UNNEST(territory_systems) FROM npc_factions WHERE galaxy_id = $1
		 )`,
		f.GalaxyID,
	)
	if err != nil {
		slog.Error("npc engine: expansion query", "faction", f.ID, "err", err)
		return
	}

	var unclaimedSystems []string
	for rows.Next() {
		var sysID string
		if err := rows.Scan(&sysID); err == nil {
			unclaimedSystems = append(unclaimedSystems, sysID)
		}
	}
	rows.Close()

	if len(unclaimedSystems) == 0 {
		return
	}

	// Pick one randomly.
	newSystem := unclaimedSystems[rng.Intn(len(unclaimedSystems))]

	// Update the faction's territory.
	_, err = e.pool.Exec(ctx,
		`UPDATE npc_factions SET territory_systems = array_append(territory_systems, $1) WHERE id = $2`,
		newSystem, f.ID,
	)
	if err != nil {
		slog.Error("npc engine: expansion update", "faction", f.ID, "err", err)
		return
	}

	payloadEN := fmt.Sprintf("%s has extended its influence into the %s system.", f.Name, newSystem)
	payloadDE := fmt.Sprintf("%s hat seinen Einfluss auf das System %s ausgedehnt.", f.NameDE, newSystem)
	e.insertWorldEvent(ctx, f.ID, "expansion", f.GalaxyID, newSystem, payloadEN, payloadDE, tickNumber)
}

// runRaid handles the aggressive raid behavior.
func (e *Engine) runRaid(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	// Pick a random system from the galaxy.
	var systemID string
	err := e.pool.QueryRow(ctx,
		`SELECT system_id FROM planets WHERE galaxy_id = $1 ORDER BY RANDOM() LIMIT 1`,
		f.GalaxyID,
	).Scan(&systemID)
	if err != nil {
		slog.Error("npc engine: raid pick system", "faction", f.ID, "err", err)
		return
	}
	_ = rng // rng used for probability check in caller; RANDOM() used in DB query

	payloadEN := fmt.Sprintf("%s raid detected in %s! Civilians are at risk.", f.Name, systemID)
	payloadDE := fmt.Sprintf("%s-Überfall in %s gemeldet! Zivilisten sind in Gefahr.", f.NameDE, systemID)
	e.insertWorldEvent(ctx, f.ID, "raid", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)
}

// runWraithCulling handles the Wraith culling special event.
func (e *Engine) runWraithCulling(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	if len(f.TerritorySystems) == 0 {
		return
	}
	systemID := f.TerritorySystems[rng.Intn(len(f.TerritorySystems))]

	payloadEN := fmt.Sprintf("A Wraith hive ship has initiated a culling operation in the %s system. All inhabitants are in grave danger.", systemID)
	payloadDE := fmt.Sprintf("Ein Wraith-Mutterschiff hat eine Aussonderungsoperation im System %s eingeleitet. Alle Bewohner sind in großer Gefahr.", systemID)
	e.insertWorldEvent(ctx, f.ID, "culling", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)

	// Wraith fed on humans — increase fleet strength.
	_, err := e.pool.Exec(ctx,
		`UPDATE npc_factions SET fleet_strength = fleet_strength + 10 WHERE id = 'wraith'`,
	)
	if err != nil {
		slog.Error("npc engine: wraith culling fleet update", "err", err)
	}
}

// runReplicatorIncursion handles the Replicator incursion special event.
func (e *Engine) runReplicatorIncursion(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	// Pick any system from the galaxy.
	var systemID string
	err := e.pool.QueryRow(ctx,
		`SELECT system_id FROM planets WHERE galaxy_id = $1 ORDER BY RANDOM() LIMIT 1`,
		f.GalaxyID,
	).Scan(&systemID)
	if err != nil {
		slog.Error("npc engine: replicator incursion pick system", "err", err)
		return
	}
	_ = rng

	payloadEN := fmt.Sprintf("Replicator fragments have been detected spreading through the %s system. Quarantine advised.", systemID)
	payloadDE := fmt.Sprintf("Replikator-Fragmente wurden bei der Ausbreitung durch das System %s entdeckt. Quarantäne empfohlen.", systemID)
	e.insertWorldEvent(ctx, f.ID, "incursion", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)
}

// runRecapture attempts to reclaim a player-captured system that originally
// belonged to this faction's territory. The NPC attacks the system with its
// fleet_strength; if the system's defense_strength is lower, the NPC wins.
func (e *Engine) runRecapture(ctx context.Context, f faction, tickNumber int64, rng *rand.Rand) {
	// Pick a random territory system that is currently player-controlled.
	systemIdx := rng.Intn(len(f.TerritorySystems))
	systemID := f.TerritorySystems[systemIdx]

	var controllerType string
	var defenseStrength int
	err := e.pool.QueryRow(ctx,
		`SELECT controller_type, defense_strength FROM system_control
		 WHERE system_id = $1 AND galaxy_id = $2`,
		systemID, f.GalaxyID,
	).Scan(&controllerType, &defenseStrength)
	if err != nil || controllerType != "player" {
		// System not player-held — nothing to recapture.
		return
	}

	// NPC attacks the defense.
	attackRoll := rng.Intn(f.FleetStrength + 1)
	defenseRoll := rng.Intn(defenseStrength + 1)

	if attackRoll > defenseRoll {
		// NPC wins — reclaim system.
		_, err := e.pool.Exec(ctx,
			`UPDATE system_control
			 SET controller_faction = $1,
			     controller_type    = 'npc',
			     defense_strength   = $2,
			     updated_at         = NOW()
			 WHERE system_id = $3 AND galaxy_id = $4`,
			f.ID, f.FleetStrength/2, systemID, f.GalaxyID,
		)
		if err != nil {
			slog.Error("npc engine: recapture update", "faction", f.ID, "system", systemID, "err", err)
			return
		}
		payloadEN := fmt.Sprintf("%s forces have recaptured the %s system, expelling all defenders.", f.Name, systemID)
		payloadDE := fmt.Sprintf("%s-Kräfte haben das System %s zurückerobert und alle Verteidiger vertrieben.", f.NameDE, systemID)
		e.insertWorldEvent(ctx, f.ID, "system_recaptured", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)
	} else {
		// NPC repelled — reduce attack impact.
		payloadEN := fmt.Sprintf("%s assault on %s was repelled. The system holds.", f.Name, systemID)
		payloadDE := fmt.Sprintf("Der Angriff von %s auf %s wurde abgewehrt. Das System hält stand.", f.NameDE, systemID)
		e.insertWorldEvent(ctx, f.ID, "system_assault_repelled", f.GalaxyID, systemID, payloadEN, payloadDE, tickNumber)
	}
}

// insertWorldEvent inserts a world_events row.
func (e *Engine) insertWorldEvent(ctx context.Context, factionID, eventType, galaxyID, systemID, payloadEN, payloadDE string, tickNumber int64) {
	_, err := e.pool.Exec(ctx,
		`INSERT INTO world_events (faction_id, event_type, galaxy_id, system_id, payload_en, payload_de, tick_number, is_public)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)`,
		factionID, eventType, galaxyID, systemID, payloadEN, payloadDE, tickNumber,
	)
	if err != nil {
		slog.Error("npc engine: insert world_event", "faction", factionID, "type", eventType, "err", err)
	}
}
