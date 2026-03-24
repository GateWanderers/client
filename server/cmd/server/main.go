package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"gatewanderers/server/internal/api"
	"gatewanderers/server/internal/auth"
	"gatewanderers/server/internal/config"
	"gatewanderers/server/internal/db"
	"gatewanderers/server/internal/economy"
	"gatewanderers/server/internal/galaxy"
	"gatewanderers/server/internal/hub"
	"gatewanderers/server/internal/npc"
	"gatewanderers/server/internal/registry"
	"gatewanderers/server/internal/ticker"
)

func main() {
	// Configure structured logging. JSON in production, text locally.
	var logHandler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	if os.Getenv("LOG_FORMAT") == "json" {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(logHandler))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config.Load failed", "err", err)
		os.Exit(1)
	}

	// Run database migrations.
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("db.RunMigrations failed", "err", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")

	// Connect to the database pool.
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db.Connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database connected")

	// Seed galaxy data (idempotent — skips if already seeded).
	seeder := galaxy.NewSeeder(pool)
	if err := seeder.Seed(ctx); err != nil {
		slog.Error("galaxy.Seed failed", "err", err)
		os.Exit(1)
	}
	slog.Info("galaxy seeded")

	// Seed economy data (idempotent — skips if already seeded).
	econSeeder := economy.NewSeeder(pool)
	if err := econSeeder.Seed(ctx); err != nil {
		slog.Error("economy.Seed failed", "err", err)
		os.Exit(1)
	}
	slog.Info("economy seeded")

	// Seed NPC factions (idempotent — skips if already seeded).
	npcSeeder := npc.NewSeeder(pool)
	if err := npcSeeder.Seed(ctx); err != nil {
		slog.Error("npc.Seed failed", "err", err)
		os.Exit(1)
	}
	slog.Info("npc factions seeded")

	// Seed system control (idempotent — must run after galaxy + NPC seeders).
	if err := seeder.SeedSystemControl(ctx); err != nil {
		slog.Error("galaxy.SeedSystemControl failed", "err", err)
		os.Exit(1)
	}
	slog.Info("system control seeded")

	// Seed mining nodes (idempotent — must run after galaxy seeder).
	if err := seeder.SeedMiningNodes(ctx); err != nil {
		slog.Error("galaxy.SeedMiningNodes failed", "err", err)
		os.Exit(1)
	}
	slog.Info("mining nodes seeded")

	// Create the PASETO token maker.
	tokenMaker, err := auth.NewTokenMaker(cfg.PasetoKey)
	if err != nil {
		slog.Error("auth.NewTokenMaker failed", "err", err)
		os.Exit(1)
	}

	// Create the WebSocket hub.
	h := hub.New()

	// Create and start the game ticker (interval configurable via TICK_INTERVAL env, default 60s).
	tickInterval := 60 * time.Second
	if v := os.Getenv("TICK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			tickInterval = d
		}
	}
	npcEngine := npc.NewEngine(pool)
	t := ticker.New(pool, h, npcEngine, tickInterval)
	t.Start(ctx)
	defer t.Stop()

	// Build the server and router.
	reg := registry.New(pool)
	srv := api.NewServer(tokenMaker, reg, h, t)
	router := srv.NewRouter()

	addr := fmt.Sprintf(":%s", cfg.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // no write timeout for WebSocket connections
		IdleTimeout:  60 * time.Second,
	}

	// Start in a goroutine so we can listen for shutdown signals.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("http server panic recovered",
					"panic", r,
					"stack", string(debug.Stack()),
				)
				os.Exit(1)
			}
		}()
		slog.Info("server listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ListenAndServe failed", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT / SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
