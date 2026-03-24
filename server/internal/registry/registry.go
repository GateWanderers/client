package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Registry provides database operations for accounts, agents and ships.
type Registry struct {
	pool *pgxpool.Pool
}

// New creates a Registry backed by the given connection pool.
func New(pool *pgxpool.Pool) *Registry {
	return &Registry{pool: pool}
}

// Pool exposes the underlying connection pool for direct queries.
func (r *Registry) Pool() *pgxpool.Pool {
	return r.pool
}

// ErrEmailTaken is returned when an email address is already registered.
var ErrEmailTaken = errors.New("email already registered")

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidCredentials is returned for bad login attempts.
var ErrInvalidCredentials = errors.New("invalid credentials")

// RegisterInput holds all fields needed to create an account, agent and ship.
type RegisterInput struct {
	Email        string
	PasswordHash string
	Language     string
	AgentName    string
	Faction      string
	Playstyle    string
}

// Register creates an account, agent and starter ship in a single transaction.
func (r *Registry) Register(ctx context.Context, in RegisterInput) (*Account, *Agent, *Ship, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Insert account.
	var acc Account
	err = tx.QueryRow(ctx,
		`INSERT INTO accounts (email, password_hash, language)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, language, created_at`,
		in.Email, in.PasswordHash, in.Language,
	).Scan(&acc.ID, &acc.Email, &acc.PasswordHash, &acc.Language, &acc.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, nil, nil, ErrEmailTaken
		}
		return nil, nil, nil, fmt.Errorf("insert account: %w", err)
	}

	// Insert agent.
	var agent Agent
	err = tx.QueryRow(ctx,
		`INSERT INTO agents (account_id, name, faction, playstyle)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, account_id, name, faction, playstyle, credits, experience,
		           skills, research, reputation, mission_brief, status, created_at`,
		acc.ID, in.AgentName, in.Faction, in.Playstyle,
	).Scan(
		&agent.ID, &agent.AccountID, &agent.Name, &agent.Faction, &agent.Playstyle,
		&agent.Credits, &agent.Experience,
		&agent.Skills, &agent.Research, &agent.Reputation,
		&agent.MissionBrief, &agent.Status, &agent.CreatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert agent: %w", err)
	}

	// Insert starter ship.
	var ship Ship
	err = tx.QueryRow(ctx,
		`INSERT INTO ships (agent_id)
		 VALUES ($1)
		 RETURNING id, agent_id, name, class, hull_points, max_hull_points,
		           galaxy_id, system_id, planet_id, equipment, created_at`,
		agent.ID,
	).Scan(
		&ship.ID, &ship.AgentID, &ship.Name, &ship.Class,
		&ship.HullPoints, &ship.MaxHullPoints,
		&ship.GalaxyID, &ship.SystemID, &ship.PlanetID,
		&ship.Equipment, &ship.CreatedAt,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("insert ship: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("commit tx: %w", err)
	}

	return &acc, &agent, &ship, nil
}

// GetAccountByEmail fetches a full account row by email address.
func (r *Registry) GetAccountByEmail(ctx context.Context, email string) (*Account, error) {
	var acc Account
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, language, created_at
		 FROM accounts WHERE email = $1`,
		email,
	).Scan(&acc.ID, &acc.Email, &acc.PasswordHash, &acc.Language, &acc.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query account by email: %w", err)
	}
	return &acc, nil
}

// GetAgentState fetches the agent and its first ship for a given account ID.
func (r *Registry) GetAgentState(ctx context.Context, accountID string) (*Agent, *Ship, error) {
	var agent Agent
	err := r.pool.QueryRow(ctx,
		`SELECT id, account_id, name, faction, playstyle, credits, experience,
		        skills, research, reputation, mission_brief, status, created_at
		 FROM agents WHERE account_id = $1`,
		accountID,
	).Scan(
		&agent.ID, &agent.AccountID, &agent.Name, &agent.Faction, &agent.Playstyle,
		&agent.Credits, &agent.Experience,
		&agent.Skills, &agent.Research, &agent.Reputation,
		&agent.MissionBrief, &agent.Status, &agent.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("query agent: %w", err)
	}

	var ship Ship
	err = r.pool.QueryRow(ctx,
		`SELECT s.id, s.agent_id, s.name, s.class, s.hull_points, s.max_hull_points,
		        s.galaxy_id, s.system_id, s.planet_id, s.equipment, s.created_at,
		        s.cargo_capacity,
		        COALESCE((SELECT SUM(quantity) FROM inventories WHERE agent_id = s.agent_id), 0) AS cargo_used
		 FROM ships s WHERE s.agent_id = $1
		 ORDER BY s.created_at ASC LIMIT 1`,
		agent.ID,
	).Scan(
		&ship.ID, &ship.AgentID, &ship.Name, &ship.Class,
		&ship.HullPoints, &ship.MaxHullPoints,
		&ship.GalaxyID, &ship.SystemID, &ship.PlanetID,
		&ship.Equipment, &ship.CreatedAt,
		&ship.CargoCapacity, &ship.CargoUsed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		return nil, nil, fmt.Errorf("query ship: %w", err)
	}

	return &agent, &ship, nil
}

// GetAlliances returns all alliance rows (in either direction) involving the given agent ID.
func (r *Registry) GetAlliances(ctx context.Context, agentID string) ([]Alliance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, proposer_id, target_id, status, created_at
		 FROM alliances
		 WHERE proposer_id = $1 OR target_id = $1
		 ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query alliances: %w", err)
	}
	defer rows.Close()

	var alliances []Alliance
	for rows.Next() {
		var a Alliance
		if err := rows.Scan(&a.ID, &a.ProposerID, &a.TargetID, &a.Status, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alliance: %w", err)
		}
		alliances = append(alliances, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("alliances rows: %w", err)
	}
	if alliances == nil {
		alliances = []Alliance{}
	}
	return alliances, nil
}

// isUniqueViolation returns true when err is a PostgreSQL unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
