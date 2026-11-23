#!/usr/bin/env bash
# Prepare a fresh Linux VM for LaunchBase production (vendor-neutral).
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  echo "run as root (sudo)" >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

if command -v apt-get >/dev/null 2>&1; then
  apt-get update -y
  apt-get install -y ca-certificates curl gnupg ufw git
  if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
  fi
  systemctl enable --now docker
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y curl firewalld git
  if ! command -v docker >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
  fi
  systemctl enable --now docker
else
  echo "install Docker Engine manually, then re-run" >&2
  exit 1
fi

if command -v ufw >/dev/null 2>&1; then
  ufw default deny incoming
  ufw default allow outgoing
  ufw allow OpenSSH
  ufw allow 80/tcp
  ufw allow 443/tcp
  ufw --force enable
  ufw status
elif command -v firewall-cmd >/dev/null 2>&1; then
  systemctl enable --now firewalld
  firewall-cmd --permanent --add-service=ssh
  firewall-cmd --permanent --add-service=http
  firewall-cmd --permanent --add-service=https
  firewall-cmd --reload
fi

install -d -m 0750 /etc/launchbase
install -d -m 0750 /var/backups/launchbase
install -d -m 0755 /opt/launchbase

echo
echo "Next:"
echo "  1. Clone LaunchBase into /opt/launchbase"
echo "  2. Copy infrastructure/production/.env.example to infrastructure/production/.env"
echo "  3. Place the GitHub App PEM at the path in GITHUB_APP_PRIVATE_KEY_HOST_PATH (mode 600)"
echo "  4. Point DNS A/AAAA for CONTROL_HOST and *.BASE_DOMAIN at this VM"
echo "  5. cd /opt/launchbase/infrastructure/production && docker compose --env-file .env up -d --build"
echo "  6. Install cron from docs/deployment.md (backup + cleanup)"
