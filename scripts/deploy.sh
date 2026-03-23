#!/usr/bin/env bash
# scripts/deploy.sh
# Pull latest code, build, migrate, and restart services.
# Usage:  bash scripts/deploy.sh [--branch main]

set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn()  { echo -e "${YELLOW}[$(date +%H:%M:%S)]${NC} $*"; }
error() { echo -e "${RED}[$(date +%H:%M:%S)]${NC} $*"; exit 1; }

BRANCH="${2:-main}"
COMPOSE="docker compose"

# ── 0. Sanity checks ─────────────────────────────────────────────────────────
[[ -f ".env" ]] || error ".env not found. Copy .env.example and fill in values."
command -v docker &>/dev/null || error "Docker not installed. Run scripts/init-server.sh first."

# ── 1. Pull latest code ───────────────────────────────────────────────────────
if git rev-parse --git-dir &>/dev/null; then
    info "Pulling latest code (branch: $BRANCH)..."
    git fetch origin
    git checkout "$BRANCH"
    git pull origin "$BRANCH"
    COMMIT=$(git rev-parse --short HEAD)
    info "Now at commit $COMMIT"
else
    warn "Not a git repo — skipping git pull (manual deploy mode)"
fi

# ── 2. Build image ────────────────────────────────────────────────────────────
info "Building Docker image..."
$COMPOSE build --no-cache server

# ── 3. Start / recreate services ──────────────────────────────────────────────
info "Starting postgres (if not running)..."
$COMPOSE up -d postgres

info "Waiting for postgres to be healthy..."
RETRIES=30
until docker inspect --format='{{.State.Health.Status}}' gw-postgres 2>/dev/null | grep -q "healthy"; do
    RETRIES=$((RETRIES - 1))
    [[ $RETRIES -le 0 ]] && error "Postgres did not become healthy in time."
    sleep 2
done
info "Postgres is healthy."

# ── 4. Restart server (embedded migrations run automatically on startup) ──────
info "Restarting game server..."
$COMPOSE up -d --force-recreate server

# ── 6. Start nginx ────────────────────────────────────────────────────────────
info "Ensuring nginx is running..."
$COMPOSE up -d nginx

# ── 7. Health check ──────────────────────────────────────────────────────────
info "Waiting for server to respond..."
RETRIES=20
until curl -sf http://localhost:8080/map > /dev/null 2>&1; do
    RETRIES=$((RETRIES - 1))
    [[ $RETRIES -le 0 ]] && { warn "Server did not respond in time — check logs: make logs"; break; }
    sleep 2
done

echo ""
info "══════════════════════════════════════════════"
info " Deployment complete!"
info "══════════════════════════════════════════════"
echo ""
$COMPOSE ps
echo ""
echo "  Map:    http://$(hostname -I | awk '{print $1}')/map"
echo "  Admin:  http://$(hostname -I | awk '{print $1}')/admin"
echo "  Logs:   docker compose logs -f gw-server"
echo ""
