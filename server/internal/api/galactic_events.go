package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ── GET /galactic-events ─────────────────────────────────────────────────

func (s *Server) handleGetGalacticEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	galaxy := r.URL.Query().Get("galaxy")

	rows, err := s.registry.Pool().Query(ctx,
		`SELECT id, event_type, galaxy_id, title_en, title_de,
		        description_en, description_de, effect, started_at, ends_at
		 FROM galactic_events
		 WHERE (ends_at IS NULL OR ends_at > NOW())
		   AND ($1 = '' OR galaxy_id = $1 OR galaxy_id = 'all')
		 ORDER BY started_at DESC`,
		galaxy,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch galactic events")
		return
	}
	defer rows.Close()

	type eventRow struct {
		ID            string          `json:"id"`
		EventType     string          `json:"event_type"`
		GalaxyID      string          `json:"galaxy_id"`
		TitleEN       string          `json:"title_en"`
		TitleDE       string          `json:"title_de"`
		DescriptionEN string          `json:"description_en"`
		DescriptionDE string          `json:"description_de"`
		Effect        json.RawMessage `json:"effect"`
		StartedAt     time.Time       `json:"started_at"`
		EndsAt        *time.Time      `json:"ends_at,omitempty"`
	}

	var events []eventRow
	for rows.Next() {
		var e eventRow
		if err := rows.Scan(&e.ID, &e.EventType, &e.GalaxyID,
			&e.TitleEN, &e.TitleDE, &e.DescriptionEN, &e.DescriptionDE,
			&e.Effect, &e.StartedAt, &e.EndsAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		events = append(events, e)
	}
	if events == nil {
		events = []eventRow{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events, "count": len(events)})
}

// ── POST /admin/galactic-events ───────────────────────────────────────────

var validGalacticEventTypes = map[string]bool{
	"INVASION":    true,
	"TRADE_BOOM":  true,
	"NEBULA_STORM": true,
	"ARMISTICE":   true,
	"TOURNAMENT":  true,
}

// Predefined event templates with localized text and effects.
var galacticEventTemplates = map[string]struct {
	TitleEN, TitleDE, DescEN, DescDE string
	Effect                           map[string]interface{}
}{
	"INVASION": {
		TitleEN: "Goa'uld Invasion",
		TitleDE: "Goa'uld-Invasion",
		DescEN:  "A massive Goa'uld fleet has emerged from hyperspace. NPC forces are significantly stronger.",
		DescDE:  "Eine massive Goa'uld-Flotte ist aus dem Hyperraum aufgetaucht. NPC-Streitkräfte sind deutlich stärker.",
		Effect:  map[string]interface{}{"combat_mult": 1.5, "npc_strength_mult": 2.0},
	},
	"TRADE_BOOM": {
		TitleEN: "Interstellar Trade Boom",
		TitleDE: "Interstellarer Handelsboom",
		DescEN:  "Favorable trade winds sweep the galaxy. All commodity prices doubled.",
		DescDE:  "Günstige Handelswinde durchziehen die Galaxis. Alle Warenpreise verdoppelt.",
		Effect:  map[string]interface{}{"trade_mult": 2.0},
	},
	"NEBULA_STORM": {
		TitleEN: "Subspace Nebula Storm",
		TitleDE: "Subraum-Nebelsturm",
		DescEN:  "Subspace interference disrupts gate travel. Dial success chance halved.",
		DescDE:  "Subraum-Störungen behindern Gate-Reisen. Dial-Erfolgschance halbiert.",
		Effect:  map[string]interface{}{"gate_mult": 0.5},
	},
	"ARMISTICE": {
		TitleEN: "Galactic Armistice",
		TitleDE: "Galaktischer Waffenstillstand",
		DescEN:  "A fragile peace holds across the galaxy. NPC factions stand down.",
		DescDE:  "Ein fragiler Frieden hält in der Galaxis. NPC-Fraktionen ziehen sich zurück.",
		Effect:  map[string]interface{}{"combat_mult": 0.3, "npc_strength_mult": 0.5},
	},
	"TOURNAMENT": {
		TitleEN: "Gateworld Tournament",
		TitleDE: "Gateworld-Turnier",
		DescEN:  "The Ancient Tournament begins. All XP gains doubled for the duration.",
		DescDE:  "Das Antike Turnier beginnt. Alle XP-Gewinne für die Dauer verdoppelt.",
		Effect:  map[string]interface{}{"xp_mult": 2.0},
	},
}

func (s *Server) handleAdminCreateGalacticEvent(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		EventType string  `json:"event_type"`
		GalaxyID  string  `json:"galaxy_id"`
		DurationH float64 `json:"duration_hours"` // 0 = no auto-end
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !validGalacticEventTypes[body.EventType] {
		writeError(w, http.StatusBadRequest, "valid event_type required: INVASION, TRADE_BOOM, NEBULA_STORM, ARMISTICE, TOURNAMENT")
		return
	}

	if body.GalaxyID == "" {
		body.GalaxyID = "all"
	}

	tmpl := galacticEventTemplates[body.EventType]
	effectJSON, _ := json.Marshal(tmpl.Effect)

	var endsAt *time.Time
	if body.DurationH > 0 {
		t := time.Now().Add(time.Duration(body.DurationH * float64(time.Hour)))
		endsAt = &t
	}

	var eventID string
	err := s.registry.Pool().QueryRow(ctx,
		`INSERT INTO galactic_events
		   (event_type, galaxy_id, title_en, title_de, description_en, description_de, effect, ends_at, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		body.EventType, body.GalaxyID,
		tmpl.TitleEN, tmpl.TitleDE, tmpl.DescEN, tmpl.DescDE,
		effectJSON, endsAt, adminID,
	).Scan(&eventID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create event")
		return
	}

	s.auditLog(ctx, adminID, "create_galactic_event", eventID, `{"type":"`+body.EventType+`","galaxy":"`+body.GalaxyID+`"}`)

	// Broadcast to all connected clients.
	payload, _ := json.Marshal(map[string]interface{}{
		"action":    "galactic_event_started",
		"event_id":  eventID,
		"type":      body.EventType,
		"galaxy_id": body.GalaxyID,
		"title_en":  tmpl.TitleEN,
		"title_de":  tmpl.TitleDE,
	})
	s.adminBroadcast("galactic_event", map[string]interface{}{
		"action":    "started",
		"event_id":  eventID,
		"type":      body.EventType,
		"title_en":  tmpl.TitleEN,
		"title_de":  tmpl.TitleDE,
		"galaxy_id": body.GalaxyID,
	})
	_ = payload // already included in adminBroadcast

	writeJSON(w, http.StatusCreated, map[string]interface{}{"event_id": eventID, "status": "active"})
}

// ── DELETE /admin/galactic-events/{eventID} ───────────────────────────────

func (s *Server) handleAdminEndGalacticEvent(w http.ResponseWriter, r *http.Request) {
	adminID := accountIDFromContext(r.Context())
	eventID := chi.URLParam(r, "eventID")
	ctx := r.Context()

	tag, err := s.registry.Pool().Exec(ctx,
		`UPDATE galactic_events SET ends_at = NOW() WHERE id = $1 AND (ends_at IS NULL OR ends_at > NOW())`,
		eventID,
	)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "event not found or already ended")
		return
	}

	s.auditLog(ctx, adminID, "end_galactic_event", eventID, "")
	s.adminBroadcast("galactic_event", map[string]interface{}{
		"action":   "ended",
		"event_id": eventID,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
}
