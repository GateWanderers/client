#!/usr/bin/env bash
# scripts/migrate.sh
# Applies all SQL migrations in order to the running postgres container.
# Safe to run multiple times — each migration uses IF NOT EXISTS / IF EXISTS.

set -euo pipefail

GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
info()  { echo -e "${GREEN}[migrate]${NC} $*"; }
error() { echo -e "${RED}[migrate]${NC} $*"; exit 1; }

DB_CONTAINER="${DB_CONTAINER:-gw-postgres}"
DB_USER="${POSTGRES_USER:-gatewanderers}"
DB_NAME="${POSTGRES_DB:-gatewanderers}"

# Source .env if available (for POSTGRES_USER / POSTGRES_DB overrides).
[[ -f ".env" ]] && set -o allexport && source .env && set +o allexport

MIGRATIONS_DIR="server/migrations"
[[ -d "$MIGRATIONS_DIR" ]] || error "Migrations directory not found: $MIGRATIONS_DIR"

# Check container is running.
docker inspect "$DB_CONTAINER" &>/dev/null \
  || error "Container $DB_CONTAINER is not running. Start postgres first."

info "Applying migrations from $MIGRATIONS_DIR ..."

# Apply in alphabetical order (001_, 002_, ...).
for sql in $(ls "$MIGRATIONS_DIR"/*.sql | sort); do
    name=$(basename "$sql")
    info "  → $name"
    docker cp "$sql" "$DB_CONTAINER:/tmp/$name"
    docker exec "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" \
        -v ON_ERROR_STOP=1 -f "/tmp/$name" \
        2>&1 | sed 's/^/       /'
done

info "All migrations applied."
