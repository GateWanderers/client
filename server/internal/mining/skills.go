package mining

// Skill defines an active ability with level progression.
type Skill struct {
	ID            string
	Name          string
	NameDE        string
	Description   string
	DescriptionDE string
	XPPerUse      int
	CooldownTicks int
	// LevelThresholds[i] = XP needed to reach level i+2
	// Index 0 → L2, Index 1 → L3, Index 2 → L4, Index 3 → L5
	LevelThresholds [4]int
}

// SkillRegistry lists all player-usable active skills.
var SkillRegistry = map[string]Skill{
	"overcharge_drill": {
		ID:            "overcharge_drill",
		Name:          "Overcharge Drill",
		NameDE:        "Überladener Bohrer",
		Description:   "Overloads mining equipment for a massive yield boost on the next MINE action (2×–5× by level).",
		DescriptionDE: "Überlastet die Minenausrüstung für enormen Ertrag beim nächsten MINE-Befehl (2×–5× je Level).",
		XPPerUse:      8,
		CooldownTicks: 8,
		LevelThresholds: [4]int{5, 15, 30, 50},
	},
	"deep_survey": {
		ID:            "deep_survey",
		Name:          "Deep Survey",
		NameDE:        "Tiefenscan",
		Description:   "Surveys all systems in the current galaxy. Survey duration scales with level.",
		DescriptionDE: "Scannt alle Systeme in der aktuellen Galaxie. Scandauer steigt mit Level.",
		XPPerUse:      10,
		CooldownTicks: 12,
		LevelThresholds: [4]int{5, 15, 30, 50},
	},
	"cargo_compress": {
		ID:            "cargo_compress",
		Name:          "Cargo Compress",
		NameDE:        "Laderaumkomprimierung",
		Description:   "Temporarily compresses cargo to increase hold capacity (+50–+250 by level).",
		DescriptionDE: "Komprimiert Fracht temporär, um die Ladekapazität zu erhöhen (+50–+250 je Level).",
		XPPerUse:      6,
		CooldownTicks: 15,
		LevelThresholds: [4]int{5, 15, 30, 50},
	},
	"scavenge": {
		ID:            "scavenge",
		Name:          "Scavenge",
		NameDE:        "Plündern",
		Description:   "Salvages resources from this system's available deposits, even without a proper node.",
		DescriptionDE: "Birgt Ressourcen aus verfügbaren Vorkommen in diesem System, auch ohne regulären Node.",
		XPPerUse:      5,
		CooldownTicks: 10,
		LevelThresholds: [4]int{5, 15, 30, 50},
	},
	"emergency_jettison": {
		ID:            "emergency_jettison",
		Name:          "Emergency Jettison",
		NameDE:        "Notabwurf",
		Description:   "Jettisons all cargo for 25% market value as credits — frees your hold instantly.",
		DescriptionDE: "Wirft gesamte Fracht für 25% des Marktwerts ab — leert den Laderaum sofort.",
		XPPerUse:      3,
		CooldownTicks: 20,
		LevelThresholds: [4]int{5, 15, 30, 50},
	},
}

// LevelForXP returns the skill level for the given accumulated XP.
func LevelForXP(sk Skill, xp int) int {
	level := 1
	for i, threshold := range sk.LevelThresholds {
		if xp >= threshold {
			level = i + 2
		}
	}
	return level
}

// OverchargeDrillMultiplier returns the yield multiplier for overcharge_drill at the given level.
func OverchargeDrillMultiplier(level int) float64 {
	switch level {
	case 1:
		return 2.0
	case 2:
		return 2.5
	case 3:
		return 3.0
	case 4:
		return 4.0
	default:
		return 5.0
	}
}

// CargoCompressBonus returns extra cargo units granted by cargo_compress at the given level.
func CargoCompressBonus(level int) int {
	return level * 50 // L1=+50 … L5=+250
}

// DeepSurveyDuration returns how many ticks a deep_survey result lasts at the given level.
func DeepSurveyDuration(level int) int64 {
	return int64(level * 20) // L1=20 ticks … L5=100 ticks
}

// ScavengeYield returns min/max resource units from scavenge at the given level.
func ScavengeYield(level int) (min, max int) {
	return level * 5, level * 15 // L1=5–15 … L5=25–75
}
