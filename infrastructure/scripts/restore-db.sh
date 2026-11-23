#!/usr/bin/env bash
# Restore a gzipped pg_dump into the production Postgres service.
# WARNING: replaces database contents.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /var/backups/launchbase/launchbase-YYYYMMDDTHHMMSSZ.sql.gz" >&2
  exit 2
fi
dump=$1
if [[ ! -f "$dump" ]]; then
  echo "missing dump: $dump" >&2
  exit 1
fi

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
prod="$root/infrastructure/production"
cd "$prod"

echo "Restoring $dump into compose service postgres..."
gunzip -c "$dump" | docker compose --env-file .env exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
echo "restore complete"
