#!/bin/sh
# Controlled end-to-end: build examples/node-demo, route through Traefik, assert body.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$root"

docker compose up -d traefik
cleanup() {
  docker compose stop traefik >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "waiting for Traefik..."
i=0
while [ "$i" -lt 30 ]; do
  if curl -sf http://127.0.0.1:8088/ping >/dev/null 2>&1; then
    break
  fi
  i=$((i + 1))
  sleep 1
done
if [ "$i" -eq 30 ]; then
  echo "Traefik did not become ready" >&2
  exit 1
fi

cd "$root/backend"
exec go test -tags=integration -count=1 -timeout 15m ./internal/deployments -run TestExampleNodeDemo
