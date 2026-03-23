package ticker

import (
	"fmt"
	"hash/fnv"
)

// ExploreResult holds the outcome of an EXPLORE action.
type ExploreResult struct {
	Outcome      string // "empty", "minerals", "ruins", "npc_patrol", "anomaly"
	ResourceType string // only for minerals: "Naquadah" or "Trinium"
	Amount       int    // only for minerals
	PayloadEN    string
	PayloadDE    string
}

// processExplore computes a deterministic exploration outcome from the agent's
// current system and the current tick number.
func processExplore(agentID, systemID string, tickNumber int64) ExploreResult {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s:%s:%d", agentID, systemID, tickNumber)
	seed := h.Sum64()

	bucket := int(seed % 100)

	switch {
	case bucket < 30:
		// 0-29: empty space
		return ExploreResult{
			Outcome:   "empty",
			PayloadEN: fmt.Sprintf("Your sensors sweep the %s system. Nothing but drifting dust and starlight.", systemID),
			PayloadDE: fmt.Sprintf("Ihre Sensoren durchsuchen das System %s. Nichts als treibender Staub und Sternenlicht.", systemID),
		}

	case bucket < 55:
		// 30-54: mineral deposits
		resources := []string{"Naquadah", "Trinium"}
		resType := resources[(seed>>8)%2]
		amount := 10 + int((seed>>16)%41) // 10–50
		return ExploreResult{
			Outcome:      "minerals",
			ResourceType: resType,
			Amount:       amount,
			PayloadEN:    fmt.Sprintf("Mineral survey of %s reveals %d units of %s in a nearby asteroid field.", systemID, amount, resType),
			PayloadDE:    fmt.Sprintf("Die Mineralsurvey von %s zeigt %d Einheiten %s in einem nahen Asteroidenfeld.", systemID, amount, resType),
		}

	case bucket < 70:
		// 55-69: Ancient ruins fragment
		return ExploreResult{
			Outcome:   "ruins",
			PayloadEN: fmt.Sprintf("Deep-space scan in %s picks up the unmistakable energy signature of Ancient ruins.", systemID),
			PayloadDE: fmt.Sprintf("Tiefraum-Scan in %s erfasst die unverwechselbare Energiesignatur alter Ruinen.", systemID),
		}

	case bucket < 85:
		// 70-84: NPC patrol sighted
		return ExploreResult{
			Outcome:   "npc_patrol",
			PayloadEN: fmt.Sprintf("Long-range scanners in %s detect an unidentified patrol vessel on an intercept course.", systemID),
			PayloadDE: fmt.Sprintf("Langstrecken-Scanner in %s erfassen ein nicht identifiziertes Patrouillenschiff auf Abfangkurs.", systemID),
		}

	default:
		// 85-99: anomaly
		return ExploreResult{
			Outcome:   "anomaly",
			PayloadEN: fmt.Sprintf("Strange energy readings emanate from a point source in %s. The instruments can barely keep up.", systemID),
			PayloadDE: fmt.Sprintf("Merkwürdige Energiemesswerte stammen von einer Punktquelle in %s. Die Instrumente kommen kaum hinterher.", systemID),
		}
	}
}
