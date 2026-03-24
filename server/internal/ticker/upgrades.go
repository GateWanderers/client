package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/mining"
)

// cargoCapacityForClass returns the base cargo capacity for a ship class.
func cargoCapacityForClass(class string) int {
	if cap, ok := mining.CargoCapacityByClass[class]; ok {
		return cap
	}
	return 80 // gate_runner_mk1 default
}

// upgradeResult is returned from all upgrade/repair/buy-ship actions.
type upgradeResult struct {
	PayloadEN string
	PayloadDE string
	EventType string
	Success   bool
}

// ── REPAIR ────────────────────────────────────────────────────────────────

// processRepair restores hull_points at a cost of 2 credits per HP restored.
// Parameters: {"amount": N}   — HP to restore (default: full repair)
func processRepair(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) upgradeResult {
	var p struct {
		Amount *int `json:"amount"`
	}
	_ = json.Unmarshal(params, &p)

	// Load ship.
	var shipID string
	var curHP, maxHP, credits int
	err := pool.QueryRow(ctx,
		`SELECT s.id, s.hull_points, s.max_hull_points, a.credits
		 FROM ships s JOIN agents a ON a.id = s.agent_id
		 WHERE s.agent_id = $1 ORDER BY s.created_at ASC LIMIT 1`,
		agentID,
	).Scan(&shipID, &curHP, &maxHP, &credits)
	if err != nil {
		return upgradeResult{PayloadEN: "No ship found.", PayloadDE: "Kein Schiff gefunden.", EventType: "repair_fail"}
	}

	needed := maxHP - curHP
	if needed <= 0 {
		return upgradeResult{PayloadEN: "Hull is already at full integrity.", PayloadDE: "Hülle ist bereits vollständig intakt.", EventType: "repair_skip", Success: true}
	}

	// Default: repair as much as credits allow; cap at requested amount.
	toRepair := needed
	if p.Amount != nil && *p.Amount > 0 && *p.Amount < toRepair {
		toRepair = *p.Amount
	}

	cost := toRepair * 3
	if cost > credits {
		toRepair = credits / 3
		cost = toRepair * 3
	}
	if toRepair <= 0 {
		return upgradeResult{PayloadEN: "Insufficient credits for repair.", PayloadDE: "Nicht genug Credits für Reparatur.", EventType: "repair_fail"}
	}

	_, _ = pool.Exec(ctx,
		`UPDATE ships SET hull_points = hull_points + $1 WHERE id = $2`, toRepair, shipID,
	)
	_, _ = pool.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`, cost, agentID,
	)

	return upgradeResult{
		PayloadEN: fmt.Sprintf("Repair complete. +%d HP restored. Cost: %d credits.", toRepair, cost),
		PayloadDE: fmt.Sprintf("Reparatur abgeschlossen. +%d HP wiederhergestellt. Kosten: %d Credits.", toRepair, cost),
		EventType: "repair",
		Success:   true,
	}
}

// ── UPGRADE ───────────────────────────────────────────────────────────────

// processUpgrade increases one upgrade level (weapon/shield/engine/cargo) by 1.
// Parameters: {"system": "weapon"|"shield"|"engine"|"cargo"}
// Cost: 500 * current_level² credits.
func processUpgrade(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) upgradeResult {
	var p struct {
		System string `json:"system"`
	}
	_ = json.Unmarshal(params, &p)

	col := ""
	switch p.System {
	case "weapon":
		col = "weapon_level"
	case "shield":
		col = "shield_level"
	case "engine":
		col = "engine_level"
	case "cargo":
		col = "cargo_level"
	default:
		return upgradeResult{PayloadEN: "Unknown upgrade system. Use: weapon, shield, engine, cargo.", PayloadDE: "Unbekanntes Upgrade-System. Optionen: weapon, shield, engine, cargo.", EventType: "upgrade_fail"}
	}

	// Load current level and credits.
	var shipID string
	var curLevel, credits int
	err := pool.QueryRow(ctx,
		fmt.Sprintf(`SELECT s.id, s.%s, a.credits FROM ships s JOIN agents a ON a.id = s.agent_id WHERE s.agent_id = $1 ORDER BY s.created_at ASC LIMIT 1`, col),
		agentID,
	).Scan(&shipID, &curLevel, &credits)
	if err != nil {
		return upgradeResult{PayloadEN: "No ship found.", PayloadDE: "Kein Schiff gefunden.", EventType: "upgrade_fail"}
	}

	if curLevel >= 5 {
		return upgradeResult{PayloadEN: fmt.Sprintf("%s system is already at maximum level (5).", p.System), PayloadDE: fmt.Sprintf("%s-System ist bereits auf Maximalstufe (5).", p.System), EventType: "upgrade_fail"}
	}

	cost := 500 * curLevel * curLevel
	if credits < cost {
		return upgradeResult{
			PayloadEN: fmt.Sprintf("Insufficient credits. Upgrading %s to level %d costs %d credits.", p.System, curLevel+1, cost),
			PayloadDE: fmt.Sprintf("Nicht genug Credits. Upgrade %s auf Stufe %d kostet %d Credits.", p.System, curLevel+1, cost),
			EventType: "upgrade_fail",
		}
	}

	_, _ = pool.Exec(ctx,
		fmt.Sprintf(`UPDATE ships SET %s = %s + 1 WHERE id = $1`, col, col), shipID,
	)
	_, _ = pool.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`, cost, agentID,
	)

	return upgradeResult{
		PayloadEN: fmt.Sprintf("%s system upgraded to level %d. Cost: %d credits.", p.System, curLevel+1, cost),
		PayloadDE: fmt.Sprintf("%s-System auf Stufe %d aufgewertet. Kosten: %d Credits.", p.System, curLevel+1, cost),
		EventType: "upgrade_" + p.System,
		Success:   true,
	}
}

// ── BUY_SHIP ──────────────────────────────────────────────────────────────

type shipSpec struct {
	class   string
	name    string
	hp      int
	cost    int
	nameEN  string
	nameDE  string
}

var shipSpecs = map[string]shipSpec{
	"patrol_craft": {
		class: "patrol_craft", name: "Patrol Craft", hp: 250, cost: 3000,
		nameEN: "Patrol Craft", nameDE: "Patrouillenschiff",
	},
	"destroyer": {
		class: "destroyer", name: "Destroyer", hp: 500, cost: 12000,
		nameEN: "Destroyer", nameDE: "Zerstörer",
	},
	"battlecruiser": {
		class: "battlecruiser", name: "Battlecruiser", hp: 1000, cost: 40000,
		nameEN: "Battlecruiser", nameDE: "Schlachtkreuzer",
	},
	"mining_barge": {
		class: "mining_barge", name: "Mining Barge", hp: 200, cost: 8000,
		nameEN: "Mining Barge", nameDE: "Minenbarke",
	},
	"freighter": {
		class: "freighter", name: "Freighter", hp: 350, cost: 20000,
		nameEN: "Freighter", nameDE: "Frachter",
	},
}

// processBuyShip purchases a new ship, replacing the current one.
// Parameters: {"class": "patrol_craft"|"destroyer"|"battlecruiser"}
func processBuyShip(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) upgradeResult {
	var p struct {
		Class string `json:"class"`
	}
	_ = json.Unmarshal(params, &p)

	spec, ok := shipSpecs[p.Class]
	if !ok {
		return upgradeResult{PayloadEN: "Unknown ship class. Available: patrol_craft, destroyer, battlecruiser, mining_barge, freighter.", PayloadDE: "Unbekannte Schiffsklasse. Verfügbar: patrol_craft, destroyer, battlecruiser, mining_barge, freighter.", EventType: "buy_ship_fail"}
	}

	// Load current ship and credits.
	var shipID, galaxyID, systemID string
	var credits int
	err := pool.QueryRow(ctx,
		`SELECT s.id, s.galaxy_id, s.system_id, a.credits
		 FROM ships s JOIN agents a ON a.id = s.agent_id
		 WHERE s.agent_id = $1 ORDER BY s.created_at ASC LIMIT 1`,
		agentID,
	).Scan(&shipID, &galaxyID, &systemID, &credits)
	if err != nil {
		return upgradeResult{PayloadEN: "No ship found.", PayloadDE: "Kein Schiff gefunden.", EventType: "buy_ship_fail"}
	}

	if credits < spec.cost {
		return upgradeResult{
			PayloadEN: fmt.Sprintf("Insufficient credits. %s costs %d credits.", spec.nameEN, spec.cost),
			PayloadDE: fmt.Sprintf("Nicht genug Credits. %s kostet %d Credits.", spec.nameDE, spec.cost),
			EventType: "buy_ship_fail",
		}
	}

	// Determine cargo capacity for this ship class.
	cargo := cargoCapacityForClass(spec.class)

	// Replace old ship with new one at same location.
	_, _ = pool.Exec(ctx, `DELETE FROM ships WHERE id = $1`, shipID)
	_, _ = pool.Exec(ctx,
		`INSERT INTO ships (agent_id, name, class, hull_points, max_hull_points, galaxy_id, system_id, cargo_capacity)
		 VALUES ($1, $2, $3, $4, $4, $5, $6, $7)`,
		agentID, spec.name, spec.class, spec.hp, galaxyID, systemID, cargo,
	)
	_, _ = pool.Exec(ctx,
		`UPDATE agents SET credits = credits - $1 WHERE id = $2`, spec.cost, agentID,
	)

	return upgradeResult{
		PayloadEN: fmt.Sprintf("New %s acquired. Hull: %d/%d HP. Cost: %d credits.", spec.nameEN, spec.hp, spec.hp, spec.cost),
		PayloadDE: fmt.Sprintf("Neuer %s erworben. Hülle: %d/%d HP. Kosten: %d Credits.", spec.nameDE, spec.hp, spec.hp, spec.cost),
		EventType: "buy_ship",
		Success:   true,
	}
}
