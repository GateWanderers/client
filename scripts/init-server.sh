#!/usr/bin/env bash
# scripts/init-server.sh
# First-time setup for a fresh Debian 13 (Trixie) server.
# Run as root or with sudo.
# Usage:  bash scripts/init-server.sh

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

[[ $EUID -ne 0 ]] && error "Run as root: sudo bash scripts/init-server.sh"

# ── 1. System update ─────────────────────────────────────────────────────────
info "Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

# ── 2. Essential tools ────────────────────────────────────────────────────────
info "Installing essential tools..."
apt-get install -y -qq \
    curl wget git ca-certificates gnupg lsb-release \
    ufw fail2ban htop jq unzip

# ── 3. Docker ────────────────────────────────────────────────────────────────
if ! command -v docker &>/dev/null; then
    info "Installing Docker..."
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/debian/gpg \
      | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
      > /etc/apt/sources.list.d/docker.list
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable --now docker
    info "Docker installed: $(docker --version)"
else
    info "Docker already installed: $(docker --version)"
fi

# ── 4. Add deploy user to docker group ───────────────────────────────────────
DEPLOY_USER="${SUDO_USER:-$(logname 2>/dev/null || echo '')}"
if [[ -n "$DEPLOY_USER" && "$DEPLOY_USER" != "root" ]]; then
    usermod -aG docker "$DEPLOY_USER"
    info "Added $DEPLOY_USER to docker group (re-login required)"
fi

# ── 5. Firewall (ufw) ─────────────────────────────────────────────────────────
info "Configuring firewall..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
info "Firewall rules:"
ufw status numbered

# ── 6. Fail2ban ──────────────────────────────────────────────────────────────
info "Enabling fail2ban..."
systemctl enable --now fail2ban

# ── 7. Swap (useful on low-RAM VMs) ─────────────────────────────────────────
if [[ ! -f /swapfile ]]; then
    RAM_MB=$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo)
    if [[ $RAM_MB -lt 2048 ]]; then
        info "Low RAM detected (${RAM_MB}MB), creating 2GB swap..."
        fallocate -l 2G /swapfile
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
    fi
fi

# ── 8. Clone repo (if not already present) ───────────────────────────────────
INSTALL_DIR="/opt/gatewanderers"
if [[ ! -d "$INSTALL_DIR" ]]; then
    info "Cloning repository to $INSTALL_DIR ..."
    warn "Set your repo URL below or clone manually:"
    warn "  git clone <YOUR_REPO_URL> $INSTALL_DIR"
    warn "  cd $INSTALL_DIR && cp .env.example .env && nano .env"
else
    info "Repository already present at $INSTALL_DIR"
fi

echo ""
info "═══════════════════════════════════════════════════════"
info " Server initialisation complete!"
info "═══════════════════════════════════════════════════════"
echo ""
echo "  Next steps:"
echo "  1. Clone the repo (if not done):  git clone <URL> $INSTALL_DIR"
echo "  2. Configure:                     cd $INSTALL_DIR && cp .env.example .env && nano .env"
echo "  3. Deploy:                        bash scripts/deploy.sh"
echo ""
[[ -n "$DEPLOY_USER" ]] && warn "Re-login as $DEPLOY_USER to pick up docker group membership."
