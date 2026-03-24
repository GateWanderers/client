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

	// Ancient Seeker
	if set["ancient_database"] {
		bonus += 0.10
	}
	if set["zpm_theory"] {
		bonus += 0.20
	}

	return bonus
}
