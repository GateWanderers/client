package research

// ResourceCost represents one resource cost entry for a tech.
type ResourceCost struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// Tech represents a technology in the tech tree.
type Tech struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	NameDE        string         `json:"name_de"`
	Description   string         `json:"description"`
	DescriptionDE string         `json:"description_de"`
	Faction       string         `json:"faction"` // "all" or faction name
	TicksRequired int            `json:"ticks_required"`
	Cost          []ResourceCost `json:"cost"`
	Prerequisites []string       `json:"prerequisites"`
	Effects       string         `json:"effects"`
	EffectsDE     string         `json:"effects_de"`
}

// Registry is the global tech tree.
var Registry = map[string]Tech{
	// Universal techs (faction="all")
	"basic_navigation": {
		ID:            "basic_navigation",
		Name:          "Basic Navigation",
		NameDE:        "Grundlegende Navigation",
		Description:   "Foundational navigation techniques for traversing the gate network.",
		DescriptionDE: "Grundlegende Navigationstechniken für das Reisen durch das Torenetzwerk.",
		Faction:       "all",
		TicksRequired: 5,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 20}},
		Prerequisites: []string{},
		Effects:       "+10% gather yield",
		EffectsDE:     "+10% Sammelertrag",
	},
	"advanced_sensors": {
		ID:            "advanced_sensors",
		Name:          "Advanced Sensors",
		NameDE:        "Erweiterte Sensoren",
		Description:   "Enhanced sensor arrays for deep space scanning and threat detection.",
		DescriptionDE: "Verbesserte Sensorarrays für Tiefenraumscans und Bedrohungserkennung.",
		Faction:       "all",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 30}},
		Prerequisites: []string{"basic_navigation"},
		Effects:       "+10% gather yield, +5% trade credits",
		EffectsDE:     "+10% Sammelertrag, +5% Handelserlös",
	},
	"shield_tech": {
		ID:            "shield_tech",
		Name:          "Shield Technology",
		NameDE:        "Schildtechnologie",
		Description:   "Defensive shield systems to protect your vessel in combat.",
		DescriptionDE: "Defensive Schutzsysteme zum Schutz deines Schiffes im Kampf.",
		Faction:       "all",
		TicksRequired: 10,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 50}},
		Prerequisites: []string{},
		Effects:       "+15% combat shield strength",
		EffectsDE:     "+15% Kampfschildstärke",
	},
	"weapons_upgrade_1": {
		ID:            "weapons_upgrade_1",
		Name:          "Weapons Upgrade I",
		NameDE:        "Waffenupgrade I",
		Description:   "First-tier weapons enhancement increasing offensive capability.",
		DescriptionDE: "Erste Waffenverbesserungsstufe zur Erhöhung der Angriffskraft.",
		Faction:       "all",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 40}},
		Prerequisites: []string{},
		Effects:       "+15% weapon damage",
		EffectsDE:     "+15% Waffenschaden",
	},

	// Tau'ri techs (faction="tau_ri")
	"ancient_interface": {
		ID:            "ancient_interface",
		Name:          "Ancient Interface",
		NameDE:        "Alte Schnittstelle",
		Description:   "Interface technology allowing interaction with Ancient systems and databases.",
		DescriptionDE: "Schnittstellentechnologie zur Interaktion mit alten Systemen und Datenbanken.",
		Faction:       "tau_ri",
		TicksRequired: 15,
		Cost:          []ResourceCost{{Type: "ancient_tech", Amount: 100}},
		Prerequisites: []string{"advanced_sensors"},
		Effects:       "+15% gather yield",
		EffectsDE:     "+15% Sammelertrag",
	},
	"f302_specs": {
		ID:            "f302_specs",
		Name:          "F-302 Fighter Specs",
		NameDE:        "F-302-Jägerpläne",
		Description:   "Classified schematics for the advanced F-302 space superiority fighter.",
		DescriptionDE: "Geheime Pläne für den fortgeschrittenen F-302 Weltraumüberlegenheitsjäger.",
		Faction:       "tau_ri",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 60}},
		Prerequisites: []string{"weapons_upgrade_1"},
		Effects:       "+10% weapon damage",
		EffectsDE:     "+10% Waffenschaden",
	},
	"daedalus_class": {
		ID:            "daedalus_class",
		Name:          "Daedalus-Class Design",
		NameDE:        "Daedalus-Klasse Entwurf",
		Description:   "Engineering plans for the powerful Daedalus-class battlecruiser.",
		DescriptionDE: "Ingenieurspläne für den mächtigen Daedalus-Klasse Kampfkreuzer.",
		Faction:       "tau_ri",
		TicksRequired: 20,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 100}, {Type: "trinium", Amount: 50}},
		Prerequisites: []string{"f302_specs", "shield_tech"},
		Effects:       "+10% weapon damage, +10% shield",
		EffectsDE:     "+10% Waffenschaden, +10% Schild",
	},

	// Free Jaffa techs (faction="free_jaffa")
	"symbiote_enhancement": {
		ID:            "symbiote_enhancement",
		Name:          "Symbiote Enhancement",
		NameDE:        "Symbioten-Verbesserung",
		Description:   "Techniques to enhance Jaffa capabilities through symbiote bonding.",
		DescriptionDE: "Techniken zur Verbesserung der Jaffa-Fähigkeiten durch Symbioten-Bindung.",
		Faction:       "free_jaffa",
		TicksRequired: 10,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 50}},
		Prerequisites: []string{},
		Effects:       "+5% weapon damage, +5% shield",
		EffectsDE:     "+5% Waffenschaden, +5% Schild",
	},
	"jaffa_tactics": {
		ID:            "jaffa_tactics",
		Name:          "Jaffa Combat Tactics",
		NameDE:        "Jaffa-Kampftaktiken",
		Description:   "Ancient Jaffa military tactics refined over millennia of warfare.",
		DescriptionDE: "Alte Jaffa-Militärtaktiken, verfeinert über Jahrtausende des Krieges.",
		Faction:       "free_jaffa",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 30}},
		Prerequisites: []string{},
		Effects:       "+10% weapon damage",
		EffectsDE:     "+10% Waffenschaden",
	},
	"ha_tak_refit": {
		ID:            "ha_tak_refit",
		Name:          "Ha'tak Refit",
		NameDE:        "Ha'tak-Umbau",
		Description:   "Comprehensive refit of the Ha'tak mothership with Free Jaffa modifications.",
		DescriptionDE: "Umfassender Umbau des Ha'tak-Mutterschiffs mit Freie-Jaffa-Modifikationen.",
		Faction:       "free_jaffa",
		TicksRequired: 18,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 80}},
		Prerequisites: []string{"jaffa_tactics", "shield_tech"},
		Effects:       "+10% combat shield strength",
		EffectsDE:     "+10% Kampfschildstärke",
	},

	// Gate Nomad techs (faction="gate_nomad")
	"black_market_contacts": {
		ID:            "black_market_contacts",
		Name:          "Black Market Contacts",
		NameDE:        "Schwarzmarkt-Kontakte",
		Description:   "Network of black market traders providing access to rare goods.",
		DescriptionDE: "Netzwerk von Schwarzmarkthändlern, das Zugang zu seltenen Waren bietet.",
		Faction:       "gate_nomad",
		TicksRequired: 6,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 20}},
		Prerequisites: []string{},
		Effects:       "+15% trade credits",
		EffectsDE:     "+15% Handelserlös",
	},
	"stealth_systems": {
		ID:            "stealth_systems",
		Name:          "Stealth Systems",
		NameDE:        "Tarnungssysteme",
		Description:   "Advanced stealth technology allowing ships to evade detection.",
		DescriptionDE: "Fortschrittliche Tarnungstechnologie, die es Schiffen ermöglicht, Entdeckung zu vermeiden.",
		Faction:       "gate_nomad",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 40}},
		Prerequisites: []string{"basic_navigation"},
		Effects:       "+10% gather yield",
		EffectsDE:     "+10% Sammelertrag",
	},
	"smuggler_hold": {
		ID:            "smuggler_hold",
		Name:          "Smuggler Hold",
		NameDE:        "Schmugglerfach",
		Description:   "Hidden cargo compartments for transporting contraband undetected.",
		DescriptionDE: "Versteckte Laderäume zum unentdeckten Transport von Schmuggelware.",
		Faction:       "gate_nomad",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 20}},
		Prerequisites: []string{"black_market_contacts"},
		Effects:       "+20% gather yield, +10% trade credits",
		EffectsDE:     "+20% Sammelertrag, +10% Handelserlös",
	},

	// System Lord Remnant techs (faction="system_lord_remnant")
	"goa_uld_symbiosis": {
		ID:            "goa_uld_symbiosis",
		Name:          "Goa'uld Symbiosis",
		NameDE:        "Goa'uld-Symbiose",
		Description:   "Mastery of Goa'uld symbiote technology for enhanced host capabilities.",
		DescriptionDE: "Beherrschung der Goa'uld-Symbioten-Technologie für verbesserte Wirtsfähigkeiten.",
		Faction:       "system_lord_remnant",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 60}},
		Prerequisites: []string{},
		Effects:       "+5% weapon, +5% shield, +10% trade",
		EffectsDE:     "+5% Waffe, +5% Schild, +10% Handel",
	},
	"sarcophagus_tech": {
		ID:            "sarcophagus_tech",
		Name:          "Sarcophagus Technology",
		NameDE:        "Sarkophag-Technologie",
		Description:   "Ancient Goa'uld sarcophagus technology for healing and revival.",
		DescriptionDE: "Alte Goa'uld-Sarkophag-Technologie zur Heilung und Wiederbelebung.",
		Faction:       "system_lord_remnant",
		TicksRequired: 15,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 50}, {Type: "ancient_tech", Amount: 30}},
		Prerequisites: []string{},
		Effects:       "+15% combat shield strength",
		EffectsDE:     "+15% Kampfschildstärke",
	},
	"death_glider_upgrade": {
		ID:            "death_glider_upgrade",
		Name:          "Death Glider Upgrade",
		NameDE:        "Todesflieger-Upgrade",
		Description:   "Upgraded Death Glider fighters with enhanced weapons and shields.",
		DescriptionDE: "Aufgerüstete Todesflieger-Jäger mit verbesserten Waffen und Schilden.",
		Faction:       "system_lord_remnant",
		TicksRequired: 10,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 40}},
		Prerequisites: []string{"weapons_upgrade_1"},
		Effects:       "+10% weapon damage",
		EffectsDE:     "+10% Waffenschaden",
	},

	// Wraith Brood techs (faction="wraith_brood")
	"wraith_enzyme": {
		ID:            "wraith_enzyme",
		Name:          "Wraith Enzyme Synthesis",
		NameDE:        "Wraith-Enzym-Synthese",
		Description:   "Synthesis of the Wraith enzyme for enhanced speed and strength.",
		DescriptionDE: "Synthese des Wraith-Enzyms für verbesserte Geschwindigkeit und Stärke.",
		Faction:       "wraith_brood",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 30}},
		Prerequisites: []string{},
		Effects:       "+10% weapon damage",
		EffectsDE:     "+10% Waffenschaden",
	},
	"hive_mind_link": {
		ID:            "hive_mind_link",
		Name:          "Hive Mind Link",
		NameDE:        "Schwarmgeist-Verbindung",
		Description:   "Neural link technology connecting Wraith to the collective hive mind.",
		DescriptionDE: "Neurale Verbindungstechnologie, die Wraith mit dem kollektiven Schwarmgeist verbindet.",
		Faction:       "wraith_brood",
		TicksRequired: 15,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 50}},
		Prerequisites: []string{"advanced_sensors"},
		Effects:       "+10% gather yield",
		EffectsDE:     "+10% Sammelertrag",
	},
	"culling_beam": {
		ID:            "culling_beam",
		Name:          "Culling Beam Enhancement",
		NameDE:        "Auslesestrahl-Verbesserung",
		Description:   "Enhanced culling beam technology for more efficient harvesting.",
		DescriptionDE: "Verbesserte Auslesestrahl-Technologie für effizientere Ernte.",
		Faction:       "wraith_brood",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 60}},
		Prerequisites: []string{"weapons_upgrade_1"},
		Effects:       "+15% weapon damage",
		EffectsDE:     "+15% Waffenschaden",
	},

	// ── Mining techs ────────────────────────────────────────────────────────

	// Universal mining
	"basic_mining_tech": {
		ID:            "basic_mining_tech",
		Name:          "Basic Mining Technology",
		NameDE:        "Grundlegende Minetechnologie",
		Description:   "Foundational ore extraction techniques for improved MINE yields.",
		DescriptionDE: "Grundlegende Erzabbautechniken für verbesserte MINE-Erträge.",
		Faction:       "all",
		TicksRequired: 6,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 25}},
		Prerequisites: []string{"basic_navigation"},
		Effects:       "+15% MINE yield",
		EffectsDE:     "+15% MINE-Ertrag",
	},
	"geological_survey": {
		ID:            "geological_survey",
		Name:          "Geological Survey",
		NameDE:        "Geologische Erkundung",
		Description:   "Advanced scanning tech that extends survey validity by 50%.",
		DescriptionDE: "Fortgeschrittene Scantechnik, die die Survey-Gültigkeit um 50% verlängert.",
		Faction:       "all",
		TicksRequired: 8,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 20}},
		Prerequisites: []string{"advanced_sensors"},
		Effects:       "Survey duration ×1.5",
		EffectsDE:     "Survey-Dauer ×1.5",
	},

	// Gate Nomad mining
	"bulk_hauler": {
		ID:            "bulk_hauler",
		Name:          "Bulk Hauler Conversion",
		NameDE:        "Massentransporter-Umbau",
		Description:   "Retrofits any ship with hidden cargo pods, adding +200 cargo capacity.",
		DescriptionDE: "Rüstet jedes Schiff mit versteckten Ladepods nach, +200 Ladekapazität.",
		Faction:       "gate_nomad",
		TicksRequired: 10,
		Cost:          []ResourceCost{{Type: "trinium", Amount: 50}},
		Prerequisites: []string{"smuggler_hold"},
		Effects:       "+200 cargo capacity",
		EffectsDE:     "+200 Ladekapazität",
	},

	// Tau'ri mining
	"naquadah_drill": {
		ID:            "naquadah_drill",
		Name:          "Naquadah Drill Array",
		NameDE:        "Naquadah-Bohreranlage",
		Description:   "Specialized drilling rigs optimized for naquadah and naquadriah extraction (+25% yield).",
		DescriptionDE: "Spezialisierte Bohranlagen für Naquadah und Naquadriah-Abbau (+25% Ertrag).",
		Faction:       "tau_ri",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 60}, {Type: "trinium", Amount: 20}},
		Prerequisites: []string{"ancient_interface"},
		Effects:       "+25% naquadah/naquadriah MINE yield",
		EffectsDE:     "+25% Naquadah/Naquadriah MINE-Ertrag",
	},

	// Ancient Seeker mining
	"ancient_extraction": {
		ID:            "ancient_extraction",
		Name:          "Ancient Extraction Matrix",
		NameDE:        "Alte Extraktionsmatrix",
		Description:   "Ancient resonance technology for exceptional ancient_tech extraction (+30% yield, passive mini-gather per tick).",
		DescriptionDE: "Alte Resonanztechnologie für außergewöhnlichen Ancient-Tech-Abbau (+30% Ertrag, passives Mini-Gather pro Tick).",
		Faction:       "ancient_seeker",
		TicksRequired: 18,
		Cost:          []ResourceCost{{Type: "ancient_tech", Amount: 150}},
		Prerequisites: []string{"ancient_database"},
		Effects:       "+30% ancient_tech yield, passive gather",
		EffectsDE:     "+30% Ancient-Tech-Ertrag, passives Sammeln",
	},

	// Free Jaffa mining
	"strip_mining": {
		ID:            "strip_mining",
		Name:          "Strip Mining Protocol",
		NameDE:        "Abbauprotokoll",
		Description:   "Aggressive extraction: 2× MINE yield but 2× node depletion.",
		DescriptionDE: "Aggressiver Abbau: 2× MINE-Ertrag, aber 2× Node-Erschöpfung.",
		Faction:       "free_jaffa",
		TicksRequired: 10,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 40}},
		Prerequisites: []string{"jaffa_tactics"},
		Effects:       "2× MINE yield + 2× depletion",
		EffectsDE:     "2× MINE-Ertrag + 2× Erschöpfung",
	},

	// System Lord Remnant mining
	"automated_harvesters": {
		ID:            "automated_harvesters",
		Name:          "Automated Harvesters",
		NameDE:        "Automatisierte Erntemaschinerie",
		Description:   "Drone harvesters passively gather a small amount of resources each tick.",
		DescriptionDE: "Drohnen-Ernter sammeln passiv jede Runde eine kleine Menge Ressourcen.",
		Faction:       "system_lord_remnant",
		TicksRequired: 15,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 80}},
		Prerequisites: []string{"death_glider_upgrade"},
		Effects:       "Passive gather 3–8 units/tick",
		EffectsDE:     "Passives Sammeln 3–8 Einheiten/Tick",
	},

	// Wraith Brood mining
	"bio_extraction": {
		ID:            "bio_extraction",
		Name:          "Bio-Extraction Technique",
		NameDE:        "Bio-Extraktionstechnik",
		Description:   "Wraith bio-tech harvests bonus resources from defeated NPCs.",
		DescriptionDE: "Wraith-Biotechnik erntet Bonusressourcen von besiegten NPCs.",
		Faction:       "wraith_brood",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "naquadah", Amount: 50}},
		Prerequisites: []string{"hive_mind_link"},
		Effects:       "+10–30 resources on NPC kill",
		EffectsDE:     "+10–30 Ressourcen bei NPC-Kill",
	},

	// Ancient Seeker techs (faction="ancient_seeker")
	"ancient_database": {
		ID:            "ancient_database",
		Name:          "Ancient Database Access",
		NameDE:        "Zugang zur Alten Datenbank",
		Description:   "Access to the vast Ancient database of knowledge and technology.",
		DescriptionDE: "Zugang zur riesigen Alten Wissensdatenbank und Technologie.",
		Faction:       "ancient_seeker",
		TicksRequired: 12,
		Cost:          []ResourceCost{{Type: "ancient_tech", Amount: 100}},
		Prerequisites: []string{"advanced_sensors"},
		Effects:       "+10% gather yield",
		EffectsDE:     "+10% Sammelertrag",
	},
	"zpm_theory": {
		ID:            "zpm_theory",
		Name:          "ZPM Theory",
		NameDE:        "ZPM-Theorie",
		Description:   "Theoretical understanding of Zero Point Module energy generation.",
		DescriptionDE: "Theoretisches Verständnis der Nullpunkt-Modul-Energieerzeugung.",
		Faction:       "ancient_seeker",
		TicksRequired: 20,
		Cost:          []ResourceCost{{Type: "ancient_tech", Amount: 200}},
		Prerequisites: []string{"ancient_database"},
		Effects:       "+15% shield, +20% gather yield",
		EffectsDE:     "+15% Schild, +20% Sammelertrag",
	},
	"lantean_shielding": {
		ID:            "lantean_shielding",
		Name:          "Lantean Shielding",
		NameDE:        "Lanteanische Schirmung",
		Description:   "Advanced Lantean shield technology surpassing conventional shields.",
		DescriptionDE: "Fortschrittliche Lanteanische Schildtechnologie, die konventionelle Schilde übertrifft.",
		Faction:       "ancient_seeker",
		TicksRequired: 15,
		Cost:          []ResourceCost{{Type: "ancient_tech", Amount: 80}},
		Prerequisites: []string{"shield_tech"},
		Effects:       "+25% combat shield strength",
		EffectsDE:     "+25% Kampfschildstärke",
	},
}

// Get returns a tech by ID. Second return is false if not found.
func Get(id string) (Tech, bool) {
	t, ok := Registry[id]
	return t, ok
}

// Available returns techs the agent can research:
// - not already completed
// - not currently in progress
// - faction matches (faction=="all" or matches agent faction)
// - all prerequisites completed
func Available(agentFaction string, completed []string, inProgress string) []Tech {
	completedSet := make(map[string]bool, len(completed))
	for _, id := range completed {
		completedSet[id] = true
	}

	var result []Tech
	for _, tech := range Registry {
		// Skip if already completed.
		if completedSet[tech.ID] {
			continue
		}
		// Skip if currently in progress.
		if tech.ID == inProgress {
			continue
		}
		// Check faction compatibility.
		if tech.Faction != "all" && tech.Faction != agentFaction {
			continue
		}
		// Check all prerequisites are completed.
		allPrereqsMet := true
		for _, prereq := range tech.Prerequisites {
			if !completedSet[prereq] {
				allPrereqsMet = false
				break
			}
		}
		if !allPrereqsMet {
			continue
		}
		result = append(result, tech)
	}
	return result
}

// CanResearch checks prerequisites, faction, and whether the tech is not already done.
// Returns (true, "") if the tech can be researched, or (false, reason) if not.
func CanResearch(tech Tech, agentFaction string, completed []string) (bool, string) {
	completedSet := make(map[string]bool, len(completed))
	for _, id := range completed {
		completedSet[id] = true
	}

	// Check if already completed.
	if completedSet[tech.ID] {
		return false, "tech already researched"
	}

	// Check faction compatibility.
	if tech.Faction != "all" && tech.Faction != agentFaction {
		return false, "tech not available for your faction"
	}

	// Check prerequisites.
	for _, prereq := range tech.Prerequisites {
		if !completedSet[prereq] {
			return false, "missing prerequisite: " + prereq
		}
	}

	return true, ""
}
