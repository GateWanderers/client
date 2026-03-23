package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"gatewanderers/server/internal/ratelimit"
)

type contextKey string

const contextKeyAccountID contextKey = "account_id"

// jsonMiddleware sets Content-Type: application/json on every response.
func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates the Bearer PASETO token and injects account_id into context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing Authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeError(w, http.StatusUnauthorized, "Authorization header must be 'Bearer <token>'")
			return
		}

		accountID, err := s.tokenMaker.VerifyToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Reject banned agents even with a valid token.
		var isBanned bool
		if err := s.registry.Pool().QueryRow(r.Context(),
			`SELECT banned_at IS NOT NULL FROM agents WHERE account_id = $1`, accountID,
		).Scan(&isBanned); err == nil && isBanned {
			writeError(w, http.StatusForbidden, "account is banned")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyAccountID, accountID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// accountIDFromContext extracts the account ID set by authMiddleware.
func accountIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyAccountID).(string)
	return v
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// rateLimitMiddleware rejects requests that exceed the given limiter's quota.
// The key is the real client IP, extracted from X-Forwarded-For when present.
func rateLimitMiddleware(lim *ratelimit.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !lim.Allow(ip) {
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded — please slow down")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realIP returns the client's real IP, preferring X-Forwarded-For.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address — that's the original client.
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		return addr[:i]
	}
	return addr
}

// writeJSON writes a JSON success response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
