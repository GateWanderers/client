package ticker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/jackc/pgx/v5/pgxpool"

	"gatewanderers/server/internal/research"
)

// CombatRound records one round of combat.
type CombatRound struct {
	Round     int `json:"round"`
	PlayerDmg int `json:"player_dmg"`
	NPCDmg    int `json:"npc_dmg"`
	PlayerHP  int `json:"player_hp"`
	NPCHP     int `json:"npc_hp"`
}

// LootItem represents a salvaged resource.
type LootItem struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// CombatResult holds the full outcome of an ATTACK action.
type CombatResult struct {
	Outcome       string // "victory", "defeat", "retreat", "no_hostiles"
	Rounds        []CombatRound
	Loot          []LootItem
	ShipDestroyed bool
	PayloadEN     string
	PayloadDE     string
	NPCFaction    string
	NPCStrength   int
}

// npcEntry is used to unmarshal npc_presence JSONB rows.
type npcEntry struct {
	Faction  string `json:"faction"`
	Strength int    `json:"strength"`
}

// shipCombatData holds the ship fields needed for combat.
type shipCombatData struct {
	ID            string
	HullPoints    int
	MaxHullPoints int
	GalaxyID      string
	SystemID      string
	PlanetID      *string
	WeaponLevel   int
	ShieldLevel   int
}

// AgentFleetMember bundles one attacker's IDs and ship for fleet combat.
type AgentFleetMember struct {
	AgentID   string
	AccountID string
	Ship      shipCombatData
}

// FleetMemberResult holds per-agent outcome after fleet combat.
type FleetMemberResult struct {
	AgentID       string
	AccountID     string
	ShipDestroyed bool
	PayloadEN     string
	PayloadDE     string
}

// FleetCombatResult holds the full outcome of a fleet ATTACK.
type FleetCombatResult struct {
	Outcome     string // "victory", "defeat", "retreat", "no_hostiles"
	Rounds      []CombatRound
	Loot        []LootItem
	NPCFaction  string
	NPCStrength int
	Members     []FleetMemberResult
}

// processAttack handles an ATTACK action.
// Parameters JSON: {"target": "npc"} — in Phase 4 only NPC targets.
func processAttack(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage, tickNumber int64, rng *rand.Rand) CombatResult {
	// 1. Load agent's ship data.
	var ship shipCombatData
	err := pool.QueryRow(ctx,
		`SELECT id, hull_points, max_hull_points, galaxy_id, system_id, planet_id,
		        weapon_level, shield_level
		 FROM ships WHERE agent_id = $1
		 ORDER BY created_at ASC LIMIT 1`,
		agentID,
	).Scan(&ship.ID, &ship.HullPoints, &ship.MaxHullPoints, &ship.GalaxyID, &ship.SystemID, &ship.PlanetID,
		&ship.WeaponLevel, &ship.ShieldLevel)
	if err != nil {
		return CombatResult{
			Outcome:   "no_hostiles",
			PayloadEN: "Combat aborted: ship not found.",
			PayloadDE: "Kampf abgebrochen: Schiff nicht gefunden.",
		}
	}

	// Resolve system name for payload text.
	var systemName string
	_ = pool.QueryRow(ctx,
		`SELECT system_name FROM planets WHERE system_id = $1 LIMIT 1`,
		ship.SystemID,
	).Scan(&systemName)
	if systemName == "" {
		systemName = ship.SystemID
	}

	// 2. Load planet's npc_presence.
	var npcPresenceRaw []byte
	if ship.PlanetID != nil {
		err = pool.QueryRow(ctx,
			`SELECT npc_presence FROM planets WHERE id = $1`,
			*ship.PlanetID,
		).Scan(&npcPresenceRaw)
	} else {
		err = pool.QueryRow(ctx,
			`SELECT npc_presence FROM planets WHERE system_id = $1 LIMIT 1`,
			ship.SystemID,
		).Scan(&npcPresenceRaw)
	}
	if err != nil {
		return CombatResult{
			Outcome:   "no_hostiles",
			PayloadEN: fmt.Sprintf("Sensors show no hostile forces in the vicinity of %s.", systemName),
			PayloadDE: fmt.Sprintf("Sensoren zeigen keine feindlichen Kräfte in der Nähe von %s.", systemName),
		}
	}

	// 3. Parse npc_presence and check for hostiles.
	var npcs []npcEntry
	if err := json.Unmarshal(npcPresenceRaw, &npcs); err != nil || len(npcs) == 0 {
		return CombatResult{
			Outcome:   "no_hostiles",
			PayloadEN: fmt.Sprintf("Sensors show no hostile forces in the vicinity of %s.", systemName),
			PayloadDE: fmt.Sprintf("Sensoren zeigen keine feindlichen Kräfte in der Nähe von %s.", systemName),
		}
	}

	npc := npcs[0]

	// 4. Run combat simulation.
	playerHP := ship.HullPoints
	npcHP := npc.Strength * 2
	var rounds []CombatRound
	outcome := "defeat"

	// Load agent's completed research for combat bonuses.
	var researchRaw []byte
	_ = pool.QueryRow(ctx,
		`SELECT research FROM agents WHERE id = $1`, agentID,
	).Scan(&researchRaw)
	var completedResearch []string
	_ = json.Unmarshal(researchRaw, &completedResearch)
	researchWeaponBonus, researchShieldBonus := research.CombatBonuses(completedResearch)

	// Upgrade multipliers: each level above 1 adds 25% to attack / effective HP.
	// Research bonuses are added on top.
	weaponMult := 1.0 + 0.25*float64(ship.WeaponLevel-1) + researchWeaponBonus
	shieldMult := 1.0 + 0.25*float64(ship.ShieldLevel-1) + researchShieldBonus
	effectiveMaxHP := int(float64(ship.MaxHullPoints) * shieldMult)

	for round := 1; round <= 5; round++ {
		// Player attacks NPC — boosted by weapon_level.
		playerDmgBase := float64(10+ship.MaxHullPoints/10) * weaponMult
		playerDmg := int(playerDmgBase * (0.8 + rng.Float64()*0.4))
		npcHP -= playerDmg

		// NPC attacks player.
		npcDmgBase := float64(npc.Strength / 4)
		npcDmg := int(npcDmgBase * (0.8 + rng.Float64()*0.4))
		playerHP -= npcDmg

		if playerHP < 0 {
			playerHP = 0
		}
		if npcHP < 0 {
			npcHP = 0
		}

		rounds = append(rounds, CombatRound{
			Round:     round,
			PlayerDmg: playerDmg,
			NPCDmg:    npcDmg,
			PlayerHP:  playerHP,
			NPCHP:     npcHP,
		})

		// Check retreat threshold: playerHP < 25% of effective max HP.
		if playerHP < effectiveMaxHP/4 && npcHP > 0 && playerHP > 0 {
			outcome = "retreat"
			// Update ship hull_points to damaged value.
			_, _ = pool.Exec(ctx,
				`UPDATE ships SET hull_points = $1 WHERE id = $2`,
				playerHP, ship.ID,
			)
			result := CombatResult{
				Outcome:     "retreat",
				Rounds:      rounds,
				NPCFaction:  npc.Faction,
				NPCStrength: npc.Strength,
				PayloadEN:   fmt.Sprintf("Tactical retreat from %s forces in %s. Ship hull at %d/%d.", npc.Faction, systemName, playerHP, ship.MaxHullPoints),
				PayloadDE:   fmt.Sprintf("Taktischer Rückzug vor %s-Streitkräften in %s. Schiffshülle bei %d/%d.", npc.Faction, systemName, playerHP, ship.MaxHullPoints),
			}
			storeCombatLog(ctx, pool, agentID, npc, ship, rounds, result, tickNumber)
			updateFactionReputation(ctx, pool, agentID, npc.Faction)
			return result
		}

		if npcHP <= 0 {
			outcome = "victory"
			break
		}
		if playerHP <= 0 {
			outcome = "defeat"
			break
		}
	}

	// If we reached max rounds without a break condition, determine by HP.
	if outcome == "defeat" && npcHP <= 0 {
		outcome = "victory"
	}

	switch outcome {
	case "victory":
		// Generate loot — tiered by NPC strength.
		var lootType string
		var lootAmount int
		switch {
		case npc.Strength >= 70:
			// High-tier: ancient_tech, naquadriah, or naquadah
			lootChoice := rng.Intn(5)
			if lootChoice < 2 {
				lootType = "ancient_tech"
			} else if lootChoice < 3 {
				lootType = "naquadriah"
			} else {
				lootType = "naquadah"
			}
			lootAmount = 20 + rng.Intn(31) // 20-50
		case npc.Strength >= 40:
			// Mid-tier: naquadah, trinium, or ancient_tech
			lootChoice := rng.Intn(5)
			if lootChoice < 1 {
				lootType = "ancient_tech"
			} else if lootChoice < 3 {
				lootType = "naquadah"
			} else {
				lootType = "trinium"
			}
			lootAmount = 15 + rng.Intn(26) // 15-40
		default:
			// Low-tier: naquadah or trinium
			if rng.Intn(2) == 0 {
				lootType = "naquadah"
			} else {
				lootType = "trinium"
			}
			lootAmount = 10 + rng.Intn(21) // 10-30
		}

		loot := []LootItem{{Type: lootType, Amount: lootAmount}}

		// Insert loot into agent's inventory.
		_, _ = pool.Exec(ctx,
			`INSERT INTO inventories (agent_id, resource_type, quantity)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (agent_id, resource_type) DO UPDATE
			   SET quantity = inventories.quantity + EXCLUDED.quantity`,
			agentID, lootType, lootAmount,
		)

		// bio_extraction bonus: extra resources from defeated NPC.
		if research.HasBioExtraction(completedResearch) {
			bioAmount := 10 + rng.Intn(21) // 10–30 extra
			_, _ = pool.Exec(ctx,
				`INSERT INTO inventories (agent_id, resource_type, quantity)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (agent_id, resource_type) DO UPDATE
				   SET quantity = inventories.quantity + EXCLUDED.quantity`,
				agentID, lootType, bioAmount,
			)
			lootAmount += bioAmount
		}

		// Award XP for combat victory.
		_, _ = pool.Exec(ctx,
			`UPDATE agents SET experience = experience + 35 WHERE id = $1`, agentID,
		)

		result := CombatResult{
			Outcome:     "victory",
			Rounds:      rounds,
			Loot:        loot,
			NPCFaction:  npc.Faction,
			NPCStrength: npc.Strength,
			PayloadEN:   fmt.Sprintf("Victory! Your ship defeated the %s patrol in %s. Salvaged %d units of %s.", npc.Faction, systemName, lootAmount, lootType),
			PayloadDE:   fmt.Sprintf("Sieg! Dein Schiff hat die %s-Patrouille in %s besiegt. %d Einheiten %s geborgen.", npc.Faction, systemName, lootAmount, lootType),
		}
		applySystemCombatDamage(ctx, pool, agentID, ship.SystemID, ship.GalaxyID, tickNumber)
		storeCombatLog(ctx, pool, agentID, npc, ship, rounds, result, tickNumber)
		updateFactionReputation(ctx, pool, agentID, npc.Faction)
		return result

	default: // "defeat"
		// Award XP for combat defeat (consolation).
		_, _ = pool.Exec(ctx,
			`UPDATE agents SET experience = experience + 5 WHERE id = $1`, agentID,
		)

		// Destroy ship; keep 75% of credits (min 100); schedule auto-respawn.
		_, _ = pool.Exec(ctx,
			`UPDATE ships SET hull_points = 0 WHERE id = $1`,
			ship.ID,
		)
		_, _ = pool.Exec(ctx,
			`UPDATE agents
			 SET status       = 'rescue_pod',
			     credits      = GREATEST(FLOOR(credits * 0.60)::int, 250),
			     death_tick   = $2,
			     respawn_tick = $2 + $3
			 WHERE id = $1`,
			agentID, tickNumber, respawnDelay,
		)
		// Pay out any active bounties on this agent to the NPCs (no player killer here).
		_, _ = pool.Exec(ctx,
			`UPDATE bounties SET status = 'expired'
			 WHERE target_id = $1 AND status = 'active'`,
			agentID,
		)

		result := CombatResult{
			Outcome:       "defeat",
			Rounds:        rounds,
			ShipDestroyed: true,
			NPCFaction:    npc.Faction,
			NPCStrength:   npc.Strength,
			PayloadEN:     fmt.Sprintf("Your ship was destroyed by %s forces in %s. You escaped in a rescue pod. Credits lost.", npc.Faction, systemName),
			PayloadDE:     fmt.Sprintf("Dein Schiff wurde von %s-Streitkräften in %s zerstört. Du entkamst in einer Rettungskapsel. Credits verloren.", npc.Faction, systemName),
		}
		storeCombatLog(ctx, pool, agentID, npc, ship, rounds, result, tickNumber)
		updateFactionReputation(ctx, pool, agentID, npc.Faction)
		return result
	}
}

// updateFactionReputation decrements the agent's reputation with a given NPC faction by 10.
func updateFactionReputation(ctx context.Context, pool *pgxpool.Pool, agentID, factionKey string) {
	_, err := pool.Exec(ctx,
		`UPDATE agents SET reputation = jsonb_set(
			reputation,
			array[$1],
			to_jsonb(GREATEST(-100, COALESCE((reputation->>$1)::int, 0) - 10))
		) WHERE id = $2`,
		factionKey, agentID,
	)
	if err != nil {
		slog.Error("ticker: updateFactionReputation", "agent", agentID, "faction", factionKey, "err", err)
	}
}

// loadNPC loads the first NPC entry from a planet or system and returns it plus the system name.
func loadNPC(ctx context.Context, pool *pgxpool.Pool, ship shipCombatData) (npcEntry, string, bool) {
	var systemName string
	_ = pool.QueryRow(ctx,
		`SELECT system_name FROM planets WHERE system_id = $1 LIMIT 1`,
		ship.SystemID,
	).Scan(&systemName)
	if systemName == "" {
		systemName = ship.SystemID
	}

	var npcPresenceRaw []byte
	var err error
	if ship.PlanetID != nil {
		err = pool.QueryRow(ctx,
			`SELECT npc_presence FROM planets WHERE id = $1`, *ship.PlanetID,
		).Scan(&npcPresenceRaw)
	} else {
		err = pool.QueryRow(ctx,
			`SELECT npc_presence FROM planets WHERE system_id = $1 LIMIT 1`, ship.SystemID,
		).Scan(&npcPresenceRaw)
	}
	if err != nil {
		return npcEntry{}, systemName, false
	}

	var npcs []npcEntry
	if err := json.Unmarshal(npcPresenceRaw, &npcs); err != nil || len(npcs) == 0 {
		return npcEntry{}, systemName, false
	}
	return npcs[0], systemName, true
}

// processFleetAttack resolves an ATTACK for a group of allied agents at the same location.
// Each member's ship contributes to the combined fleet stats. Damage is distributed
// proportionally. All members share the same outcome.
func processFleetAttack(ctx context.Context, pool *pgxpool.Pool, members []AgentFleetMember, tickNumber int64, rng *rand.Rand) FleetCombatResult {
	if len(members) == 0 {
		return FleetCombatResult{Outcome: "no_hostiles"}
	}

	// Use the first member's ship to locate the NPC.
	npc, systemName, hasHostiles := loadNPC(ctx, pool, members[0].Ship)
	if !hasHostiles {
		results := make([]FleetMemberResult, len(members))
		for i, m := range members {
			results[i] = FleetMemberResult{
				AgentID:   m.AgentID,
				AccountID: m.AccountID,
				PayloadEN: fmt.Sprintf("Fleet sensors show no hostile forces in the vicinity of %s.", systemName),
				PayloadDE: fmt.Sprintf("Flottensensoren zeigen keine feindlichen Kräfte in der Nähe von %s.", systemName),
			}
		}
		return FleetCombatResult{Outcome: "no_hostiles", Members: results}
	}

	// Compute combined fleet stats.
	fleetHP := 0
	fleetMaxHP := 0
	fleetAttackBase := 0
	for _, m := range members {
		fleetHP += m.Ship.HullPoints
		fleetMaxHP += m.Ship.MaxHullPoints
		fleetAttackBase += 10 + m.Ship.MaxHullPoints/10
	}

	npcHP := npc.Strength * 2
	playerHP := fleetHP
	var rounds []CombatRound
	outcome := "defeat"

	for round := 1; round <= 5; round++ {
		playerDmg := int(float64(fleetAttackBase) * (0.8 + rng.Float64()*0.4))
		npcHP -= playerDmg

		npcDmgBase := float64(npc.Strength / 4)
		npcDmg := int(npcDmgBase * (0.8 + rng.Float64()*0.4))
		playerHP -= npcDmg

		if playerHP < 0 {
			playerHP = 0
		}
		if npcHP < 0 {
			npcHP = 0
		}

		rounds = append(rounds, CombatRound{
			Round:     round,
			PlayerDmg: playerDmg,
			NPCDmg:    npcDmg,
			PlayerHP:  playerHP,
			NPCHP:     npcHP,
		})

		if playerHP < fleetMaxHP/5 && npcHP > 0 && playerHP > 0 {
			outcome = "retreat"
			break
		}
		if npcHP <= 0 {
			outcome = "victory"
			break
		}
		if playerHP <= 0 {
			outcome = "defeat"
			break
		}
	}
	if outcome == "defeat" && npcHP <= 0 {
		outcome = "victory"
	}

	// Distribute remaining HP proportionally back to each ship.
	// ratio = surviving fleet HP / total fleet HP (clamped to [0,1]).
	var hpRatio float64
	if fleetHP > 0 {
		hpRatio = float64(playerHP) / float64(fleetHP)
	}

	var loot []LootItem
	memberResults := make([]FleetMemberResult, len(members))

	switch outcome {
	case "victory":
		// Generate loot — tiered by NPC strength.
		var lootType string
		var lootTotal int
		switch {
		case npc.Strength >= 70:
			lootChoice := rng.Intn(5)
			if lootChoice < 2 {
				lootType = "ancient_tech"
			} else if lootChoice < 3 {
				lootType = "naquadriah"
			} else {
				lootType = "naquadah"
			}
			lootTotal = 20 + rng.Intn(31)
		case npc.Strength >= 40:
			lootChoice := rng.Intn(5)
			if lootChoice < 1 {
				lootType = "ancient_tech"
			} else if lootChoice < 3 {
				lootType = "naquadah"
			} else {
				lootType = "trinium"
			}
			lootTotal = 15 + rng.Intn(26)
		default:
			if rng.Intn(2) == 0 {
				lootType = "naquadah"
			} else {
				lootType = "trinium"
			}
			lootTotal = 10 + rng.Intn(21)
		}
		perAgent := lootTotal / len(members)
		if perAgent < 1 {
			perAgent = 1
		}
		loot = []LootItem{{Type: lootType, Amount: lootTotal}}

		for i, m := range members {
			newHP := int(float64(m.Ship.HullPoints) * hpRatio)
			if newHP < 1 {
				newHP = 1
			}
			_, _ = pool.Exec(ctx,
				`UPDATE ships SET hull_points = $1 WHERE id = $2`,
				newHP, m.Ship.ID,
			)
			_, _ = pool.Exec(ctx,
				`INSERT INTO inventories (agent_id, resource_type, quantity)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (agent_id, resource_type)
				 DO UPDATE SET quantity = inventories.quantity + EXCLUDED.quantity`,
				m.AgentID, lootType, perAgent,
			)
			memberResults[i] = FleetMemberResult{
				AgentID:   m.AgentID,
				AccountID: m.AccountID,
				PayloadEN: fmt.Sprintf("Fleet victory! The combined fleet defeated the %s patrol in %s. You salvaged %d units of %s.", npc.Faction, systemName, perAgent, lootType),
				PayloadDE: fmt.Sprintf("Flottensieg! Die kombinierte Flotte besiegte die %s-Patrouille in %s. Du hast %d Einheiten %s geborgen.", npc.Faction, systemName, perAgent, lootType),
			}
		}

	case "retreat":
		for i, m := range members {
			newHP := int(float64(m.Ship.HullPoints) * hpRatio)
			if newHP < 1 {
				newHP = 1
			}
			_, _ = pool.Exec(ctx,
				`UPDATE ships SET hull_points = $1 WHERE id = $2`,
				newHP, m.Ship.ID,
			)
			memberResults[i] = FleetMemberResult{
				AgentID:   m.AgentID,
				AccountID: m.AccountID,
				PayloadEN: fmt.Sprintf("Fleet tactical retreat from %s forces in %s. Your ship hull at %d/%d.", npc.Faction, systemName, newHP, m.Ship.MaxHullPoints),
				PayloadDE: fmt.Sprintf("Taktischer Flottenrückzug vor %s-Streitkräften in %s. Schiffshülle bei %d/%d.", npc.Faction, systemName, newHP, m.Ship.MaxHullPoints),
			}
		}

	default: // "defeat" — all ships destroyed
		for i, m := range members {
			_, _ = pool.Exec(ctx, `UPDATE ships SET hull_points = 0 WHERE id = $1`, m.Ship.ID)
			_, _ = pool.Exec(ctx,
				`UPDATE agents
				 SET status = 'rescue_pod',
				     credits = GREATEST(FLOOR(credits * 0.60)::int, 250),
				     death_tick = $2, respawn_tick = $2 + $3
				 WHERE id = $1`,
				m.AgentID, tickNumber, respawnDelay,
			)
			memberResults[i] = FleetMemberResult{
				AgentID:       m.AgentID,
				AccountID:     m.AccountID,
				ShipDestroyed: true,
				PayloadEN:     fmt.Sprintf("Your ship was destroyed alongside your fleet by %s forces in %s. Rescue pod deployed. Respawn in %d ticks.", npc.Faction, systemName, respawnDelay),
				PayloadDE:     fmt.Sprintf("Dein Schiff wurde von %s-Streitkräften in %s zerstört. Rettungskapsel aktiviert. Respawn in %d Ticks.", npc.Faction, systemName, respawnDelay),
			}
		}
	}

	// Store combat log for the first attacker (represents the fleet engagement).
	storeFleetCombatLog(ctx, pool, members, npc, rounds, outcome, loot, tickNumber)

	// Update faction reputation for all members.
	for _, m := range members {
		updateFactionReputation(ctx, pool, m.AgentID, npc.Faction)
	}

	return FleetCombatResult{
		Outcome:     outcome,
		Rounds:      rounds,
		Loot:        loot,
		NPCFaction:  npc.Faction,
		NPCStrength: npc.Strength,
		Members:     memberResults,
	}
}

// storeFleetCombatLog writes one combat_logs row representing the whole fleet engagement.
func storeFleetCombatLog(ctx context.Context, pool *pgxpool.Pool, members []AgentFleetMember, npc npcEntry, rounds []CombatRound, outcome string, loot []LootItem, tickNumber int64) {
	if len(members) == 0 {
		return
	}
	roundsJSON, _ := json.Marshal(rounds)
	lootJSON, _ := json.Marshal(loot)
	ship := members[0].Ship
	_, _ = pool.Exec(ctx,
		`INSERT INTO combat_logs
		 (attacker_id, defender_type, defender_id, galaxy_id, system_id, rounds, outcome, loot, tick_number)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		members[0].AgentID, "npc", npc.Faction, ship.GalaxyID, ship.SystemID,
		roundsJSON, outcome, lootJSON, tickNumber,
	)
}

// applySystemCombatDamage reduces a system's defense_strength after a combat victory.
// When defense_strength reaches 0, the system is captured by the agent's faction.
func applySystemCombatDamage(ctx context.Context, pool *pgxpool.Pool, agentID, systemID, galaxyID string, tickNumber int64) {
	const damagePerVictory = 20
	const starterDefense = 50

	// Get agent's faction.
	var faction string
	if err := pool.QueryRow(ctx,
		`SELECT faction FROM agents WHERE id = $1`, agentID,
	).Scan(&faction); err != nil {
		return
	}

	// Get current system control.
	var controllerFaction, controllerType string
	var defenseStrength int
	if err := pool.QueryRow(ctx,
		`SELECT controller_faction, controller_type, defense_strength
		 FROM system_control WHERE system_id = $1 AND galaxy_id = $2`,
		systemID, galaxyID,
	).Scan(&controllerFaction, &controllerType, &defenseStrength); err != nil {
		return
	}

	// Don't attack your own system.
	if controllerType == "player" && controllerFaction == faction {
		return
	}

	newDefense := defenseStrength - damagePerVictory
	if newDefense <= 0 {
		// System captured — flip to player faction.
		_, _ = pool.Exec(ctx,
			`UPDATE system_control
			 SET controller_faction = $1, controller_type = 'player',
			     defense_strength = $2, captured_at = $3,
			     last_contested_at = $3, updated_at = NOW()
			 WHERE system_id = $4 AND galaxy_id = $5`,
			faction, starterDefense, tickNumber, systemID, galaxyID,
		)
		// Broadcast system_captured world event.
		_, _ = pool.Exec(ctx,
			`INSERT INTO world_events
			 (faction_id, event_type, galaxy_id, system_id, payload_en, payload_de, tick_number, is_public)
			 VALUES ($1, 'system_captured', $2, $3, $4, $5, $6, true)`,
			faction,
			galaxyID,
			systemID,
			fmt.Sprintf("The %s system has been captured by the %s faction!", systemID, faction),
			fmt.Sprintf("Das System %s wurde von der Fraktion %s eingenommen!", systemID, faction),
			tickNumber,
		)
	} else {
		_, _ = pool.Exec(ctx,
			`UPDATE system_control
			 SET defense_strength = $1, last_contested_at = $2, updated_at = NOW()
			 WHERE system_id = $3 AND galaxy_id = $4`,
			newDefense, tickNumber, systemID, galaxyID,
		)
	}
}

// storeCombatLog writes a combat_logs row.
func storeCombatLog(ctx context.Context, pool *pgxpool.Pool, agentID string, npc npcEntry, ship shipCombatData, rounds []CombatRound, result CombatResult, tickNumber int64) {
	roundsJSON, _ := json.Marshal(rounds)
	lootJSON, _ := json.Marshal(result.Loot)

	_, _ = pool.Exec(ctx,
		`INSERT INTO combat_logs
		 (attacker_id, defender_type, defender_id, galaxy_id, system_id, rounds, outcome, loot, tick_number)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		agentID, "npc", npc.Faction, ship.GalaxyID, ship.SystemID,
		roundsJSON, result.Outcome, lootJSON, tickNumber,
	)
}
