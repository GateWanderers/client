package research

// CombatBonuses returns additive weapon and shield bonuses from completed research.
// These stack on top of the ship's weapon_level / shield_level multipliers in combat.
// Each bonus is a fraction (e.g. 0.15 = +15%).
func CombatBonuses(completed []string) (weaponBonus, shieldBonus float64) {
	set := make(map[string]bool, len(completed))
	for _, id := range completed {
		set[id] = true
	}

	// Universal techs
	if set["weapons_upgrade_1"] {
		weaponBonus += 0.15
	}
	if set["shield_tech"] {
		shieldBonus += 0.15
	}

	// Tau'ri
	if set["f302_specs"] {
		weaponBonus += 0.10
	}
	if set["daedalus_class"] {
		weaponBonus += 0.10
		shieldBonus += 0.10
	}

	// Free Jaffa
	if set["jaffa_tactics"] {
		weaponBonus += 0.10
	}
	if set["symbiote_enhancement"] {
		weaponBonus += 0.05
		shieldBonus += 0.05
	}
	if set["ha_tak_refit"] {
		shieldBonus += 0.10
	}

	// System Lord Remnant
	if set["death_glider_upgrade"] {
		weaponBonus += 0.10
	}
	if set["sarcophagus_tech"] {
		shieldBonus += 0.15
	}
	if set["goa_uld_symbiosis"] {
		weaponBonus += 0.05
		shieldBonus += 0.05
	}

	// Wraith Brood
	if set["wraith_enzyme"] {
		weaponBonus += 0.10
	}
	if set["culling_beam"] {
		weaponBonus += 0.15
	}

	// Ancient Seeker
	if set["lantean_shielding"] {
		shieldBonus += 0.25
	}
	if set["zpm_theory"] {
		shieldBonus += 0.15
	}

	return
}

// TradeBonus returns an additive credit multiplier from completed research.
// A return of 0.15 means +15% more credits earned per TRADE action.
func TradeBonus(completed []string) float64 {
	set := make(map[string]bool, len(completed))
	for _, id := range completed {
		set[id] = true
	}

	var bonus float64

	// Universal
	if set["advanced_sensors"] {
		bonus += 0.05 // better market intelligence
	}

	// Gate Nomad — trade-focused faction
	if set["black_market_contacts"] {
		bonus += 0.15
	}
	if set["smuggler_hold"] {
		bonus += 0.10
	}

	// System Lord Remnant
	if set["goa_uld_symbiosis"] {
		bonus += 0.10 // Goa'uld negotiation leverage
	}

	return bonus
}

// GatherBonus returns an additive yield multiplier from completed research.
// A return of 0.20 means +20% more resources gathered per GATHER action.
func GatherBonus(completed []string) float64 {
	set := make(map[string]bool, len(completed))
	for _, id := range completed {
		set[id] = true
	}

	var bonus float64

	// Universal
	if set["basic_navigation"] {
		bonus += 0.10
	}
	if set["advanced_sensors"] {
		bonus += 0.10
	}

	// Gate Nomad
	if set["smuggler_hold"] {
		bonus += 0.20
	}
	if set["black_market_contacts"] {
		bonus += 0.10
	}
	if set["stealth_systems"] {
		bonus += 0.10
	}

	// Wraith Brood
	if set["hive_mind_link"] {
		bonus += 0.10
	}

	// Tau'ri
	if set["ancient_interface"] {
		bonus += 0.15
	}

	// Ancient Seeker
	if set["ancient_database"] {
		bonus += 0.10
	}
	if set["zpm_theory"] {
		bonus += 0.20
	}

	return bonus
}

// MineBonus returns the additive yield multiplier for the MINE action.
// resourceType is the specific resource being mined (e.g. "naquadah").
// A return of 0.25 means +25% yield on top of the base amount.
func MineBonus(completed []string, resourceType string) float64 {
	set := make(map[string]bool, len(completed))
	for _, id := range completed {
		set[id] = true
	}

	var bonus float64

	// Universal
	if set["basic_mining_tech"] {
		bonus += 0.15
	}

	// Tau'ri: naquadah/naquadriah specialist
	if set["naquadah_drill"] && (resourceType == "naquadah" || resourceType == "naquadriah") {
		bonus += 0.25
	}

	// Ancient Seeker: ancient_tech specialist
	if set["ancient_extraction"] && resourceType == "ancient_tech" {
		bonus += 0.30
	}

	// Free Jaffa: strip_mining is handled separately (doubles yield AND depletion).
	// The multiplier effect is applied in mine.go, not here.

	return bonus
}

// HasStripMining reports whether the agent has researched strip_mining.
func HasStripMining(completed []string) bool {
	for _, id := range completed {
		if id == "strip_mining" {
			return true
		}
	}
	return false
}

// HasAutomatedHarvesters reports whether the agent has researched automated_harvesters.
func HasAutomatedHarvesters(completed []string) bool {
	for _, id := range completed {
		if id == "automated_harvesters" {
			return true
		}
	}
	return false
}

// HasBioExtraction reports whether the agent has researched bio_extraction.
func HasBioExtraction(completed []string) bool {
	for _, id := range completed {
		if id == "bio_extraction" {
			return true
		}
	}
	return false
}

// CargoBonus returns the flat bonus cargo capacity from completed research.
func CargoBonus(completed []string) int {
	set := make(map[string]bool, len(completed))
	for _, id := range completed {
		set[id] = true
	}

	var bonus int

	// Gate Nomad: bulk hauler conversion
	if set["bulk_hauler"] {
		bonus += 200
	}

	return bonus
}

// GeologicalSurveyBonus returns the survey duration multiplier.
// 1.0 = no bonus, 1.5 = geological_survey tech researched.
func GeologicalSurveyBonus(completed []string) float64 {
	for _, id := range completed {
		if id == "geological_survey" {
			return 1.5
		}
	}
	return 1.0
}
