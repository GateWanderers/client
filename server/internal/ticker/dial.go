package ticker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DialResult holds the outcome of a DIAL_GATE action.
type DialResult struct {
	Success    bool
	PlanetName string
	SystemName string
	GalaxyID   string
	PayloadEN  string
	PayloadDE  string
}

type dialParams struct {
	Address string `json:"address"`
}

// processDial handles a DIAL_GATE action.
// Parameters JSON: {"address": "26-05-36-11-18-23-09"}
// - Looks up planet by gate_address
// - Moves agent's ship to the planet (UPDATE ships SET galaxy_id=, system_id=, planet_id=)
// - Inserts into agent_known_planets (ON CONFLICT DO NOTHING)
// - Returns DialResult
func processDial(ctx context.Context, pool *pgxpool.Pool, agentID string, params json.RawMessage) DialResult {
	var p dialParams
	if err := json.Unmarshal(params, &p); err != nil || p.Address == "" {
		return DialResult{
			Success:   false,
			PayloadEN: "Gate dial failed: invalid address parameters.",
			PayloadDE: "Tor-Wahl fehlgeschlagen: ungültige Adressparameter.",
		}
	}

	// Look up planet by gate address.
	var planetID, planetName, systemID, systemName, galaxyID string
	err := pool.QueryRow(ctx,
		`SELECT id, name, system_id, system_name, galaxy_id
		 FROM planets WHERE gate_address = $1`,
		p.Address,
	).Scan(&planetID, &planetName, &systemID, &systemName, &galaxyID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return DialResult{
				Success:   false,
				PayloadEN: fmt.Sprintf("Gate address %s is not in the planetary database. No connection established.", p.Address),
				PayloadDE: fmt.Sprintf("Gate-Adresse %s ist nicht in der Planetendatenbank. Keine Verbindung hergestellt.", p.Address),
			}
		}
		return DialResult{
			Success:   false,
			PayloadEN: "Gate dial failed: database error.",
			PayloadDE: "Tor-Wahl fehlgeschlagen: Datenbankfehler.",
		}
	}

	// Move agent's ship to this planet.
	_, err = pool.Exec(ctx,
		`UPDATE ships SET galaxy_id = $1, system_id = $2, planet_id = $3
		 WHERE agent_id = $4`,
		galaxyID, systemID, planetID, agentID,
	)
	if err != nil {
		return DialResult{
			Success:   false,
			PayloadEN: "Gate dial failed: could not update ship location.",
			PayloadDE: "Tor-Wahl fehlgeschlagen: Schiffposition konnte nicht aktualisiert werden.",
		}
	}

	// Record planet as discovered by this agent.
	_, err = pool.Exec(ctx,
		`INSERT INTO agent_known_planets (agent_id, planet_id)
		 VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		agentID, planetID,
	)
	if err != nil {
		// Non-fatal: log but don't fail the dial.
		_ = err
	}

	return DialResult{
		Success:    true,
		PlanetName: planetName,
		SystemName: systemName,
		GalaxyID:   galaxyID,
		PayloadEN:  fmt.Sprintf("Stargate engaged. Wormhole established to %s in the %s system. Your ship has been transported.", planetName, systemName),
		PayloadDE:  fmt.Sprintf("Sterntor aktiviert. Wurmloch zu %s im System %s hergestellt. Ihr Schiff wurde transportiert.", planetName, systemName),
	}
}
