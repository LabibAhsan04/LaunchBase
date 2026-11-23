#!/usr/bin/env bash
# Dump Postgres from the production compose stack.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
prod="$root/infrastructure/production"
out_dir=${BACKUP_DIR:-/var/backups/launchbase}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$out_dir"
file="$out_dir/launchbase-$stamp.sql.gz"

cd "$prod"
docker compose --env-file .env exec -T postgres \
  sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip -c >"$file"
chmod 600 "$file"
echo "$file"

# Keep the newest 14 backups by default.
keep=${BACKUP_KEEP:-14}
ls -1t "$out_dir"/launchbase-*.sql.gz 2>/dev/null | tail -n +"$((keep + 1))" | xargs -r rm -f
