package api

import (
	"encoding/json"
	"net/http"
	"sort"

	"gatewanderers/server/internal/research"
)

// handleAgentResearch returns the agent's research state.
// GET /agent/research (auth required)
func (s *Server) handleAgentResearch(w http.ResponseWriter, r *http.Request) {
	accountID := accountIDFromContext(r.Context())
	if accountID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 1. Get agent's completed research and faction.
	var agentID, faction string
	var researchRaw []byte
	err := s.registry.Pool().QueryRow(r.Context(),
		`SELECT id, faction, research FROM agents WHERE account_id = $1`,
		accountID,
	).Scan(&agentID, &faction, &researchRaw)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	var completedList []string
	if err := json.Unmarshal(researchRaw, &completedList); err != nil {
		completedList = []string{}
	}
	if completedList == nil {
		completedList = []string{}
	}

	// 2. Get current tick from tick_state.
	var currentTick int64
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT tick_number FROM tick_state WHERE id = 1`,
	).Scan(&currentTick)

	// 3. Get in-progress research from research_queue.
	type inProgressInfo struct {
		TechID         string `json:"tech_id"`
		TechName       string `json:"tech_name"`
		StartedAtTick  int64  `json:"started_at_tick"`
		CompletesAtTick int64  `json:"completes_at_tick"`
		CurrentTick    int64  `json:"current_tick"`
		TicksRemaining int64  `json:"ticks_remaining"`
	}

	var inProgress *inProgressInfo
	var inProgressTechID string

	var dbTechID string
	var startedAtTick, completesAtTick int64
	err = s.registry.Pool().QueryRow(r.Context(),
		`SELECT tech_id, started_at_tick, completes_at_tick FROM research_queue WHERE agent_id = $1`,
		agentID,
	).Scan(&dbTechID, &startedAtTick, &completesAtTick)
	if err == nil {
		// In-progress research found.
		inProgressTechID = dbTechID
		techName := dbTechID
		if t, ok := research.Get(dbTechID); ok {
			techName = t.Name
		}
		ticksRemaining := completesAtTick - currentTick
		if ticksRemaining < 0 {
			ticksRemaining = 0
		}
		inProgress = &inProgressInfo{
			TechID:          dbTechID,
			TechName:        techName,
			StartedAtTick:   startedAtTick,
			CompletesAtTick: completesAtTick,
			CurrentTick:     currentTick,
			TicksRemaining:  ticksRemaining,
		}
	}

	// 4. Get available techs.
	availableTechs := research.Available(faction, completedList, inProgressTechID)

	// Sort available techs by ID for deterministic output.
	sort.Slice(availableTechs, func(i, j int) bool {
		return availableTechs[i].ID < availableTechs[j].ID
	})

	type availableTechItem struct {
		ID            string                  `json:"id"`
		Name          string                  `json:"name"`
		TicksRequired int                     `json:"ticks_required"`
		Cost          []research.ResourceCost `json:"cost"`
		Prerequisites []string                `json:"prerequisites"`
	}

	availableItems := make([]availableTechItem, 0, len(availableTechs))
	for _, t := range availableTechs {
		prereqs := t.Prerequisites
		if prereqs == nil {
			prereqs = []string{}
		}
		cost := t.Cost
		if cost == nil {
			cost = []research.ResourceCost{}
		}
		availableItems = append(availableItems, availableTechItem{
			ID:            t.ID,
			Name:          t.Name,
			TicksRequired: t.TicksRequired,
			Cost:          cost,
			Prerequisites: prereqs,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"completed":   completedList,
		"in_progress": inProgress,
		"available":   availableItems,
	})
}

// handleResearchTree returns the full tech registry as a JSON array.
// GET /research/tree (public, no auth)
func (s *Server) handleResearchTree(w http.ResponseWriter, r *http.Request) {
	techs := make([]research.Tech, 0, len(research.Registry))
	for _, t := range research.Registry {
		techs = append(techs, t)
	}

	// Sort by ID for deterministic output.
	sort.Slice(techs, func(i, j int) bool {
		return techs[i].ID < techs[j].ID
	})

	writeJSON(w, http.StatusOK, techs)
}
