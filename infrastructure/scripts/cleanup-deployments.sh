#!/usr/bin/env bash
# Cost control: stop READY deployments beyond MAX_READY_GLOBAL (oldest first),
# remove stopped LaunchBase containers, and prune dangling images.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
prod="$root/infrastructure/production"
cd "$prod"

cap=${MAX_READY_GLOBAL:-4}
api=${API_PUBLIC_URL:-}
if [[ -f .env ]]; then
  cap=$(grep -E '^MAX_READY_GLOBAL=' .env | head -n1 | cut -d= -f2- | tr -d '\"' | tr -d "'" || true)
  cap=${cap:-4}
  api=$(grep -E '^API_PUBLIC_URL=' .env | head -n1 | cut -d= -f2- | tr -d '\"' | tr -d "'" || true)
fi
api=${api:-http://127.0.0.1:8080}

echo "ready cap=$cap"

# List READY deployment ids oldest-first via SQL (no public listing API).
mapfile -t ready < <(docker compose --env-file .env exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "SELECT id FROM deployments WHERE status = '\''READY'\'' ORDER BY updated_at ASC"' \
  | tr -d '\r')

count=${#ready[@]}
echo "ready_count=$count"
if (( count > cap )); then
  excess=$((count - cap))
  echo "stopping $excess oldest READY deployments"
  for ((i = 0; i < excess; i++)); do
    id=${ready[$i]}
    [[ -z "$id" ]] && continue
    # Prefer docker stop of the known container name; API stop needs a session cookie.
    docker rm -f "launchbase-$id" >/dev/null 2>&1 || true
    docker compose --env-file .env exec -T postgres \
      sh -c "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"UPDATE deployments SET status = 'STOPPED', updated_at = NOW() AT TIME ZONE 'utc', finished_at = NOW() AT TIME ZONE 'utc' WHERE id = '$id' AND status = 'READY'\"" \
      >/dev/null
    echo "stopped $id"
  done
fi

# Remove exited launchbase containers and dangling images.
docker ps -aq --filter "name=launchbase-" --filter "status=exited" | xargs -r docker rm -f
docker image prune -f --filter "dangling=true" >/dev/null
echo "cleanup done (api=$api)"
