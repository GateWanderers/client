package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gatewanderers/server/internal/auth"
	"gatewanderers/server/internal/registry"
)

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	AgentName string `json:"agent_name"`
	Faction   string `json:"faction"`
	Playstyle string `json:"playstyle"`
	Language  string `json:"language"`
}

type registerResponse struct {
	Token string         `json:"token"`
	Agent *registry.Agent `json:"agent"`
}

// handleRegister creates a new account, agent and starter ship.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.AgentName = strings.TrimSpace(req.AgentName)

	if req.Email == "" || req.Password == "" || req.AgentName == "" ||
		req.Faction == "" || req.Playstyle == "" {
		writeError(w, http.StatusBadRequest, "email, password, agent_name, faction and playstyle are required")
		return
	}

	if req.Language == "" {
		req.Language = "en"
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	_, agent, _, err := s.registry.Register(r.Context(), registry.RegisterInput{
		Email:        req.Email,
		PasswordHash: passwordHash,
		Language:     req.Language,
		AgentName:    req.AgentName,
		Faction:      req.Faction,
		Playstyle:    req.Playstyle,
	})
	if err != nil {
		if errors.Is(err, registry.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	token, err := s.tokenMaker.CreateToken(agent.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		Token: token,
		Agent: agent,
	})

	// Notify admins of new registration.
	s.adminBroadcast("new_registration", map[string]interface{}{
		"agent_name": agent.Name,
		"faction":    agent.Faction,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// handleLogin authenticates an account and returns a fresh token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	acc, err := s.registry.GetAccountByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	if err := auth.CheckPassword(req.Password, acc.PasswordHash); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Reject banned agents.
	var banReason *string
	var isBanned bool
	_ = s.registry.Pool().QueryRow(r.Context(),
		`SELECT banned_at IS NOT NULL, ban_reason FROM agents WHERE account_id = $1`,
		acc.ID,
	).Scan(&isBanned, &banReason)
	if isBanned {
		msg := "account is banned"
		if banReason != nil && *banReason != "" {
			msg = "account is banned: " + *banReason
		}
		writeError(w, http.StatusForbidden, msg)
		return
	}

	token, err := s.tokenMaker.CreateToken(acc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}
