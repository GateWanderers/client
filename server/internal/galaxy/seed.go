package galaxy

// SystemSeed describes one star system and its planets.
type SystemSeed struct {
	ID      string
	Name    string
	X, Y    float32
	Planets []PlanetSeed
}

// NPCSeed describes an NPC faction present on a planet.
type NPCSeed struct {
	Faction  string `json:"faction"`
	Strength int    `json:"strength"`
}

// PlanetSeed describes one planet within a system.
type PlanetSeed struct {
	Name        string
	GateAddress string    // format: "NN-NN-NN-NN-NN-NN-NN" (7 two-digit numbers)
	Resources   []string  // e.g. ["naquadah", "trinium"]
	NPCs        []NPCSeed // NPC factions present on this planet
}

// milkyWaySystems returns the 12 Milky Way systems.
func milkyWaySystems() []SystemSeed {
	return []SystemSeed{
		{
			ID: "sol", Name: "Sol System", X: 500, Y: 500,
			Planets: []PlanetSeed{
				{Name: "Terra Nova", GateAddress: "26-05-36-11-18-23-09", Resources: []string{}},
			},
		},
		{
			ID: "abydos", Name: "Abydos", X: 420, Y: 350,
			Planets: []PlanetSeed{
				{Name: "Abydos", GateAddress: "27-07-15-32-12-03-19", Resources: []string{"naquadah"}},
			},
		},
		{
			ID: "chulak", Name: "Chulak", X: 620, Y: 310,
			Planets: []PlanetSeed{
				{Name: "Chulak", GateAddress: "08-01-29-08-22-38-14", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "jaffa_patrol", Strength: 30}}},
			},
		},
		{
			ID: "dakara", Name: "Dakara", X: 680, Y: 480,
			Planets: []PlanetSeed{
				{Name: "Dakara", GateAddress: "15-18-04-32-09-28-11", Resources: []string{"naquadah", "ancient_tech"},
					NPCs: []NPCSeed{{Faction: "goa_uld_remnant", Strength: 60}}},
			},
		},
		{
			ID: "tollana", Name: "Tollana", X: 550, Y: 200,
			Planets: []PlanetSeed{
				{Name: "Tollana", GateAddress: "33-28-07-10-04-18-25", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "replicators", Strength: 65}}},
			},
		},
		{
			ID: "edora", Name: "Edora", X: 350, Y: 450,
			Planets: []PlanetSeed{
				{Name: "Edora", GateAddress: "20-04-33-11-29-07-16", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "lucian_alliance", Strength: 38}}},
			},
		},
		{
			ID: "viz_ka", Name: "Viz'ka", X: 300, Y: 320,
			Planets: []PlanetSeed{
				{Name: "Viz'ka", GateAddress: "14-22-08-31-03-27-06", Resources: []string{"naquadriah"},
					NPCs: []NPCSeed{{Faction: "lucian_alliance", Strength: 48}}},
			},
		},
		{
			ID: "p3x_888", Name: "P3X-888", X: 450, Y: 600,
			Planets: []PlanetSeed{
				{Name: "P3X-888", GateAddress: "19-11-36-02-25-08-33", Resources: []string{"naquadah", "trinium"},
					NPCs: []NPCSeed{{Faction: "jaffa_patrol", Strength: 28}}},
			},
		},
		{
			ID: "netu", Name: "Netu", X: 700, Y: 580,
			Planets: []PlanetSeed{
				{Name: "Netu", GateAddress: "31-06-22-14-08-35-17", Resources: []string{},
					NPCs: []NPCSeed{{Faction: "goa_uld_remnant", Strength: 80}}},
			},
		},
		{
			ID: "orban", Name: "Orban", X: 580, Y: 670,
			Planets: []PlanetSeed{
				{Name: "Orban", GateAddress: "05-29-13-07-18-04-22", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "replicators", Strength: 55}}},
			},
		},
		{
			ID: "vyus", Name: "Vyus", X: 280, Y: 560,
			Planets: []PlanetSeed{
				{Name: "Vyus", GateAddress: "12-35-04-21-09-27-08", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "lucian_alliance", Strength: 42}}},
			},
		},
		{
			ID: "hebridan", Name: "Hebridan", X: 640, Y: 200,
			Planets: []PlanetSeed{
				{Name: "Hebridan", GateAddress: "06-17-31-08-25-12-36", Resources: []string{"trinium", "naquadah"},
					NPCs: []NPCSeed{{Faction: "lucian_alliance", Strength: 52}}},
			},
		},
		{
			ID: "camelot", Name: "Camelot", X: 460, Y: 270,
			Planets: []PlanetSeed{
				{Name: "Camelot", GateAddress: "11-24-06-33-09-17-28", Resources: []string{"ancient_tech"},
					NPCs: []NPCSeed{{Faction: "ori_prior", Strength: 72}}},
			},
		},
		{
			ID: "kallana", Name: "Kallana", X: 320, Y: 400,
			Planets: []PlanetSeed{
				{Name: "Kallana", GateAddress: "07-33-21-04-16-28-11", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "lucian_alliance", Strength: 45}}},
			},
		},
		{
			ID: "vis_uban", Name: "Vis Uban", X: 560, Y: 420,
			Planets: []PlanetSeed{
				{Name: "Vis Uban", GateAddress: "24-11-33-07-28-04-19", Resources: []string{"ancient_tech"}},
			},
		},
	}
}

// pegasusSystems returns the Pegasus systems.
func pegasusSystems() []SystemSeed {
	return []SystemSeed{
		{
			ID: "lantea", Name: "Lantea", X: 500, Y: 500,
			Planets: []PlanetSeed{
				{Name: "Lantea", GateAddress: "41-22-13-08-35-19-04", Resources: []string{"ancient_tech"}},
			},
		},
		{
			ID: "sateda", Name: "Sateda", X: 350, Y: 380,
			Planets: []PlanetSeed{
				{Name: "Sateda", GateAddress: "22-08-41-16-29-07-33", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 50}}},
			},
		},
		{
			ID: "hoff", Name: "Hoff", X: 620, Y: 360,
			Planets: []PlanetSeed{
				{Name: "Hoff", GateAddress: "09-33-22-04-17-41-28", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 40}}},
			},
		},
		{
			ID: "proculus", Name: "Proculus", X: 440, Y: 280,
			Planets: []PlanetSeed{
				{Name: "Proculus", GateAddress: "17-04-29-22-08-33-41", Resources: []string{},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 45}}},
			},
		},
		{
			ID: "m7g_677", Name: "M7G-677", X: 680, Y: 480,
			Planets: []PlanetSeed{
				{Name: "M7G-677", GateAddress: "33-19-08-41-22-04-11", Resources: []string{"ancient_tech"},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 35}}},
			},
		},
		{
			ID: "olesia", Name: "Olesia", X: 300, Y: 500,
			Planets: []PlanetSeed{
				{Name: "Olesia", GateAddress: "28-07-33-19-04-22-41", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "wraith_hive", Strength: 55}}},
			},
		},
		{
			ID: "taranis", Name: "Taranis", X: 560, Y: 620,
			Planets: []PlanetSeed{
				{Name: "Taranis", GateAddress: "04-22-33-08-19-41-07", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "wraith_hive", Strength: 65}}},
			},
		},
		{
			ID: "doranda", Name: "Doranda", X: 400, Y: 620,
			Planets: []PlanetSeed{
				{Name: "Doranda", GateAddress: "19-33-07-22-41-04-28", Resources: []string{"ancient_tech", "naquadah"},
					NPCs: []NPCSeed{{Faction: "wraith_hive", Strength: 90}}},
			},
		},
		{
			ID: "genia", Name: "Genia", X: 650, Y: 260,
			Planets: []PlanetSeed{
				{Name: "Genia", GateAddress: "07-41-19-33-04-22-08", Resources: []string{"naquadriah"},
					NPCs: []NPCSeed{{Faction: "replicators", Strength: 55}}},
			},
		},
		{
			ID: "belkan", Name: "Belkan", X: 250, Y: 420,
			Planets: []PlanetSeed{
				{Name: "Belkan", GateAddress: "22-04-41-07-33-08-19", Resources: []string{},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 28}}},
			},
		},
		{
			ID: "dagan", Name: "Dagan", X: 560, Y: 200,
			Planets: []PlanetSeed{
				{Name: "Dagan", GateAddress: "13-07-41-22-04-33-19", Resources: []string{"ancient_tech"},
					NPCs: []NPCSeed{{Faction: "wraith_patrol", Strength: 40}}},
			},
		},
		{
			ID: "manaria", Name: "Manaria", X: 380, Y: 520,
			Planets: []PlanetSeed{
				{Name: "Manaria", GateAddress: "08-19-04-33-41-22-07", Resources: []string{"naquadah", "trinium"}},
			},
		},
	}
}

// destinySystems returns the Destiny's Path systems.
func destinySystems() []SystemSeed {
	return []SystemSeed{
		{
			ID: "novus", Name: "Novus", X: 100, Y: 480,
			Planets: []PlanetSeed{
				{Name: "Novus", GateAddress: "51-32-14-07-28-19-03", Resources: []string{"naquadah"}},
			},
		},
		{
			ID: "eden", Name: "Eden", X: 220, Y: 500,
			Planets: []PlanetSeed{
				{Name: "Eden", GateAddress: "32-07-51-19-14-03-28", Resources: []string{"trinium", "ancient_tech"}},
			},
		},
		{
			ID: "ruins_09", Name: "Ruins-09", X: 340, Y: 470,
			Planets: []PlanetSeed{
				{Name: "Ruins-09", GateAddress: "14-51-28-32-03-07-19", Resources: []string{},
					NPCs: []NPCSeed{{Faction: "alien_threat", Strength: 32}}},
			},
		},
		{
			ID: "gravel_pit", Name: "Gravel Pit", X: 460, Y: 520,
			Planets: []PlanetSeed{
				{Name: "Gravel Pit", GateAddress: "07-19-51-14-28-03-32", Resources: []string{"naquadah"},
					NPCs: []NPCSeed{{Faction: "alien_threat", Strength: 35}}},
			},
		},
		{
			ID: "ice_world", Name: "Ice World", X: 580, Y: 490,
			Planets: []PlanetSeed{
				{Name: "Ice World", GateAddress: "28-03-19-51-07-32-14", Resources: []string{"trinium"},
					NPCs: []NPCSeed{{Faction: "alien_threat", Strength: 45}}},
			},
		},
		{
			ID: "jungle_world", Name: "Jungle World", X: 700, Y: 510,
			Planets: []PlanetSeed{
				{Name: "Jungle World", GateAddress: "03-28-07-19-51-32-14", Resources: []string{"naquadah", "trinium"},
					NPCs: []NPCSeed{{Faction: "alien_threat", Strength: 42}}},
			},
		},
		{
			ID: "desert_ruins", Name: "Desert Ruins", X: 820, Y: 480,
			Planets: []PlanetSeed{
				{Name: "Desert Ruins", GateAddress: "19-14-03-28-07-51-32", Resources: []string{"ancient_tech"},
					NPCs: []NPCSeed{{Faction: "ancient_construct", Strength: 72}}},
			},
		},
		{
			ID: "twin_suns", Name: "Twin Suns", X: 940, Y: 500,
			Planets: []PlanetSeed{
				{Name: "Twin Suns", GateAddress: "32-28-19-03-14-07-51", Resources: []string{},
					NPCs: []NPCSeed{{Faction: "ancient_construct", Strength: 85}}},
			},
		},
		{
			ID: "unnamed_outpost", Name: "Unnamed Outpost", X: 160, Y: 440,
			Planets: []PlanetSeed{
				{Name: "Unnamed Outpost", GateAddress: "03-14-28-51-19-07-32", Resources: []string{"ancient_tech"},
					NPCs: []NPCSeed{{Faction: "ancient_construct", Strength: 60}}},
			},
		},
		{
			ID: "crashed_city", Name: "Crashed City", X: 640, Y: 460,
			Planets: []PlanetSeed{
				{Name: "Crashed City", GateAddress: "28-51-07-14-03-32-19", Resources: []string{"naquadah", "ancient_tech"},
					NPCs: []NPCSeed{{Faction: "alien_threat", Strength: 52}}},
			},
		},
	}
}
