package registry

import (
	"encoding/json"
	"time"
)

// Account represents a registered user account.
type Account struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Language     string    `json:"language"`
	CreatedAt    time.Time `json:"created_at"`
}

// Agent represents a player's in-game agent.
type Agent struct {
	ID           string          `json:"id"`
	AccountID    string          `json:"account_id"`
	Name         string          `json:"name"`
	Faction      string          `json:"faction"`
	Playstyle    string          `json:"playstyle"`
	Credits      int             `json:"credits"`
	Experience   int             `json:"experience"`
	Skills       json.RawMessage `json:"skills"`
	Research     json.RawMessage `json:"research"`
	Reputation   json.RawMessage `json:"reputation"`
	MissionBrief string          `json:"mission_brief"`
	Status       string          `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
}

// Ship represents a player's ship.
type Ship struct {
	ID           string          `json:"id"`
	AgentID      string          `json:"agent_id"`
	Name         string          `json:"name"`
	Class        string          `json:"class"`
	HullPoints   int             `json:"hull_points"`
	MaxHullPoints int            `json:"max_hull_points"`
	GalaxyID     string          `json:"galaxy_id"`
	SystemID     string          `json:"system_id"`
	PlanetID     *string         `json:"planet_id"`
	Equipment    json.RawMessage `json:"equipment"`
	CreatedAt    time.Time       `json:"created_at"`
}

// Alliance represents a directional alliance record.
type Alliance struct {
	ID          string    `json:"id"`
	ProposerID  string    `json:"proposer_id"`
	TargetID    string    `json:"target_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// ResearchBonuses shows the active multipliers from completed research.
type ResearchBonuses struct {
	WeaponBonus float64 `json:"weapon_bonus"`
	ShieldBonus float64 `json:"shield_bonus"`
	GatherBonus float64 `json:"gather_bonus"`
	TradeBonus  float64 `json:"trade_bonus"`
}

// ClanInfo holds basic clan info attached to the agent state.
type ClanInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tag  string `json:"tag"`
	Role string `json:"role"`
}

// AgentState is the combined agent + ship + alliances response payload.
type AgentState struct {
	Agent           *Agent          `json:"agent"`
	Ship            *Ship           `json:"ship"`
	Alliances       []Alliance      `json:"alliances"`
	ResearchBonuses ResearchBonuses `json:"research_bonuses"`
	Clan            *ClanInfo       `json:"clan"` // nil if not in a clan
}
