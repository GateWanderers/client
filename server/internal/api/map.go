package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// systemControlEntry is one row in the galaxy control response.
type systemControlEntry struct {
	SystemID          string `json:"system_id"`
	ControllerFaction string `json:"controller_faction"`
	ControllerType    string `json:"controller_type"`
	DefenseStrength   int    `json:"defense_strength"`
	IncomePerTick     int    `json:"income_per_tick"`
}

// handleGalaxyControl serves GET /galaxy/control/{galaxyID} — public, no auth required.
// Returns territorial control data for every system in the galaxy.
func (s *Server) handleGalaxyControl(w http.ResponseWriter, r *http.Request) {
	galaxyID := chi.URLParam(r, "galaxyID")
	if galaxyID == "" {
		writeError(w, http.StatusBadRequest, "galaxyID is required")
		return
	}

	ctx := r.Context()
	pool := s.registry.Pool()

	rows, err := pool.Query(ctx,
		`SELECT system_id, controller_faction, controller_type, defense_strength, income_per_tick
		 FROM system_control
		 WHERE galaxy_id = $1
		 ORDER BY system_id`,
		galaxyID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch system control")
		return
	}
	defer rows.Close()

	var systems []systemControlEntry
	for rows.Next() {
		var e systemControlEntry
		if err := rows.Scan(&e.SystemID, &e.ControllerFaction, &e.ControllerType, &e.DefenseStrength, &e.IncomePerTick); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan control row")
			return
		}
		systems = append(systems, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "control rows error")
		return
	}
	if systems == nil {
		systems = []systemControlEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"galaxy_id": galaxyID,
		"systems":   systems,
	})
}

// planetInfo is a single planet entry in the map response.
type planetInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	GateAddress string `json:"gate_address"`
}

// systemInfo represents one star system with its planets.
type systemInfo struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	X       float64      `json:"x"`
	Y       float64      `json:"y"`
	Planets []planetInfo `json:"planets"`
}

// agentInfo represents an agent's map position.
type agentInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Faction  string  `json:"faction"`
	SystemID string  `json:"system_id"`
	PlanetID *string `json:"planet_id"`
}

// npcTerritoryEntry represents one NPC faction's territory on the map.
type npcTerritoryEntry struct {
	FactionID   string   `json:"faction_id"`
	FactionName string   `json:"faction_name"`
	Systems     []string `json:"systems"`
}

// galaxyMapResponse is the full map response payload.
type galaxyMapResponse struct {
	GalaxyID     string              `json:"galaxy_id"`
	Tick         int64               `json:"tick"`
	Systems      []systemInfo        `json:"systems"`
	Agents       []agentInfo         `json:"agents"`
	NPCTerritory []npcTerritoryEntry `json:"npc_territory"`
}

// handleGalaxyMap serves GET /galaxy/map/{galaxyID} — public, no auth required.
func (s *Server) handleGalaxyMap(w http.ResponseWriter, r *http.Request) {
	galaxyID := chi.URLParam(r, "galaxyID")
	if galaxyID == "" {
		writeError(w, http.StatusBadRequest, "galaxyID is required")
		return
	}

	ctx := r.Context()
	pool := s.registry.Pool()

	// Fetch current tick number.
	var tick int64
	if err := pool.QueryRow(ctx, `SELECT tick_number FROM tick_state WHERE id = 1`).Scan(&tick); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch tick")
		return
	}

	// Fetch all planets in this galaxy grouped by system.
	rows, err := pool.Query(ctx,
		`SELECT id, system_id, system_name, system_x, system_y, name, gate_address
		 FROM planets
		 WHERE galaxy_id = $1
		 ORDER BY system_id, name`,
		galaxyID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch planets")
		return
	}
	defer rows.Close()

	// Build systems map to group planets.
	systemsMap := make(map[string]*systemInfo)
	var systemOrder []string // preserve insertion order

	for rows.Next() {
		var (
			planetID, systemID, systemName string
			systemX, systemY               float64
			planetName, gateAddress        string
		)
		if err := rows.Scan(&planetID, &systemID, &systemName, &systemX, &systemY, &planetName, &gateAddress); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan planet row")
			return
		}

		sys, exists := systemsMap[systemID]
		if !exists {
			sys = &systemInfo{
				ID:      systemID,
				Name:    systemName,
				X:       systemX,
				Y:       systemY,
				Planets: []planetInfo{},
			}
			systemsMap[systemID] = sys
			systemOrder = append(systemOrder, systemID)
		}
		sys.Planets = append(sys.Planets, planetInfo{
			ID:          planetID,
			Name:        planetName,
			GateAddress: gateAddress,
		})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "planet rows error")
		return
	}

	// Build ordered systems slice.
	systems := make([]systemInfo, 0, len(systemOrder))
	for _, sysID := range systemOrder {
		systems = append(systems, *systemsMap[sysID])
	}

	// Fetch agents in this galaxy (via ships JOIN agents).
	agentRows, err := pool.Query(ctx,
		`SELECT a.id, a.name, a.faction, s.system_id, s.planet_id
		 FROM agents a
		 JOIN ships s ON s.agent_id = a.id
		 WHERE s.galaxy_id = $1
		 ORDER BY a.name`,
		galaxyID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch agents")
		return
	}
	defer agentRows.Close()

	var agents []agentInfo
	for agentRows.Next() {
		var ag agentInfo
		if err := agentRows.Scan(&ag.ID, &ag.Name, &ag.Faction, &ag.SystemID, &ag.PlanetID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan agent row")
			return
		}
		agents = append(agents, ag)
	}
	if err := agentRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "agent rows error")
		return
	}
	if agents == nil {
		agents = []agentInfo{}
	}

	// Fetch NPC faction territories in this galaxy.
	npcRows, err := pool.Query(ctx,
		`SELECT id, name, territory_systems FROM npc_factions
		 WHERE galaxy_id = $1 AND array_length(territory_systems, 1) > 0`,
		galaxyID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch npc territory")
		return
	}
	defer npcRows.Close()

	var npcTerritory []npcTerritoryEntry
	for npcRows.Next() {
		var entry npcTerritoryEntry
		if err := npcRows.Scan(&entry.FactionID, &entry.FactionName, &entry.Systems); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan npc territory row")
			return
		}
		npcTerritory = append(npcTerritory, entry)
	}
	if err := npcRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "npc territory rows error")
		return
	}
	if npcTerritory == nil {
		npcTerritory = []npcTerritoryEntry{}
	}

	writeJSON(w, http.StatusOK, galaxyMapResponse{
		GalaxyID:     galaxyID,
		Tick:         tick,
		Systems:      systems,
		Agents:       agents,
		NPCTerritory: npcTerritory,
	})
}
