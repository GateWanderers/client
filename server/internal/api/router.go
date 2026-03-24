package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"gatewanderers/server/internal/auth"
	"gatewanderers/server/internal/hub"
	"gatewanderers/server/internal/ratelimit"
	"gatewanderers/server/internal/registry"
	"gatewanderers/server/internal/ticker"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	tokenMaker *auth.TokenMaker
	registry   *registry.Registry
	hub        *hub.Hub
	ticker     *ticker.Ticker
}

// NewServer creates a Server with the given dependencies.
func NewServer(tokenMaker *auth.TokenMaker, reg *registry.Registry, h *hub.Hub, t *ticker.Ticker) *Server {
	return &Server{
		tokenMaker: tokenMaker,
		registry:   reg,
		hub:        h,
		ticker:     t,
	}
}

// NewRouter builds and returns the chi router with all routes registered.
func (s *Server) NewRouter() http.Handler {
	r := chi.NewRouter()

	// Rate limiters.
	// Auth endpoints: 10 attempts per minute per IP (brute-force protection).
	authLimiter := ratelimit.New(10, time.Minute)
	// General public read endpoints: 120 requests per minute per IP.
	publicLimiter := ratelimit.New(120, time.Minute)
	// Authenticated write endpoints: 60 requests per minute per IP.
	writeLimiter := ratelimit.New(60, time.Minute)

	// Global middleware.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(jsonMiddleware)

	// CORS — allow all origins for development.
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	})
	r.Use(corsHandler.Handler)

	// Public routes.
	r.With(rateLimitMiddleware(authLimiter)).Post("/auth/register", s.handleRegister)
	r.With(rateLimitMiddleware(authLimiter)).Post("/auth/login", s.handleLogin)

	// Public read endpoints (rate-limited).
	r.With(rateLimitMiddleware(publicLimiter)).Get("/galaxy/map/{galaxyID}", s.handleGalaxyMap)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/galaxy/control/{galaxyID}", s.handleGalaxyControl)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/galaxy/nodes/{systemID}", s.handleGalaxyNodes)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/events", s.handleWorldEvents)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/feed", s.handleFeed)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/market/posts", s.handleMarketPosts)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/research/tree", s.handleResearchTree)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/chat/{channel}", s.handleGetChat)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/leaderboard",    s.handleLeaderboard)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/clan/{clanID}",  s.handleGetClan)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/bounties",       s.handleGetBounties)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/galactic-events", s.handleGetGalacticEvents)
	r.With(rateLimitMiddleware(publicLimiter)).Get("/market/auctions", s.handleGetAuctions)

	// Public health check — no auth, no rate limit.
	r.Get("/health", s.handleHealth)

	// Static pages.
	r.Get("/map", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/map.html")
	})

	// WebSocket stream — auth handled inside the handler via query param or header.
	r.Get("/stream", s.handleStream)

	// Admin routes — require auth + is_admin flag.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.adminMiddleware)
		r.Get("/admin/health",   s.handleAdminHealth)
		r.Get("/admin/stats",    s.handleAdminStats)
		r.Get("/admin/clients",  s.handleAdminClients)
		r.Get("/admin/audit",    s.handleAdminAudit)
		r.Get("/admin/agents",   s.handleAdminAgents)
		r.Post("/admin/agents/{agentID}/ban",    s.handleAdminBanAgent)
		r.Delete("/admin/agents/{agentID}/ban",  s.handleAdminUnbanAgent)
		r.Post("/admin/agents/{agentID}/kick",          s.handleAdminKickAgent)
		r.Post("/admin/accounts/{accountID}/promote",   s.handleAdminPromote)
		r.Delete("/admin/accounts/{accountID}/promote", s.handleAdminDemote)
		r.Get("/admin/chat/reports",             s.handleAdminChatReports)
		r.Delete("/admin/chat/messages/{msgID}", s.handleAdminDeleteMessage)
		r.Post("/admin/tick/force",              s.handleAdminForceTick)
		r.Post("/admin/tick/pause",              s.handleAdminPauseTick)
		r.Post("/admin/tick/resume",             s.handleAdminResumeTick)
		r.Post("/admin/galactic-events",             s.handleAdminCreateGalacticEvent)
		r.Delete("/admin/galactic-events/{eventID}", s.handleAdminEndGalacticEvent)
	})

	// Admin dashboard page — public static file (auth handled client-side).
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/admin.html")
	})

	// Protected routes.
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Use(rateLimitMiddleware(writeLimiter))
		r.Get("/agent/state", s.handleAgentState)
		r.Get("/clan/mine", s.handleGetMyClan)
		r.Post("/clan", s.handleCreateClan)
		r.Delete("/clan/{clanID}", s.handleDisbandClan)
		r.Post("/clan/{clanID}/invite", s.handleClanInvite)
		r.Delete("/clan/{clanID}/member/{agentID}", s.handleClanKick)
		r.Delete("/clan/{clanID}/leave", s.handleClanLeave)
		r.Get("/agent/missions", s.handleAgentMissions)
		r.Post("/agent/action", s.handleAgentAction)
		r.Put("/agent/mission-brief", s.handleMissionBrief)
		r.Post("/agent/veto", s.handleVeto)
		r.Post("/agent/override", s.handleOverride)
		r.Get("/agent/inventory", s.handleAgentInventory)
		r.Get("/agent/research", s.handleAgentResearch)
		r.Get("/agent/surveys", s.handleAgentSurveys)
		r.Get("/agent/skills", s.handleAgentSkills)
		r.Get("/agent/reputation", s.handleAgentReputation)
		r.Get("/market/orders", s.handleMarketOrders)
		r.Post("/market/trade", s.handleCreateTradeOffer)

		// Bounties — write actions require auth.
		r.Post("/bounties",                s.handlePlaceBounty)
		r.Delete("/bounties/{bountyID}",   s.handleRetractBounty)

		// Auctions — write actions require auth.
		r.Post("/market/auction", s.handleCreateAuction)
		r.Post("/market/auction/{auctionID}/bid", s.handleBidAuction)
		r.Post("/market/auction/{auctionID}/buyout", s.handleBuyoutAuction)
		r.Delete("/market/auction/{auctionID}", s.handleCancelAuction)

		// Chat — authenticated actions.
		r.Post("/chat/{channel}", s.handlePostChat)
		r.Post("/chat/mute", s.handleMuteAgent)
		r.Delete("/chat/mute", s.handleUnmuteAgent)
		r.Get("/chat/mutes", s.handleGetMutes)
		r.Post("/chat/report", s.handleReportMessage)
	})

	return r
}
