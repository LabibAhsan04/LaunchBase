#!/usr/bin/env bash
# Operator status via the token-protected ops endpoint.
set -euo pipefail

if [[ -z "${OPS_TOKEN:-}" || -z "${API_PUBLIC_URL:-}" ]]; then
  if [[ -f infrastructure/production/.env ]]; then
    # shellcheck disable=SC1091
    set -a
    source infrastructure/production/.env
    set +a
  fi
fi

: "${OPS_TOKEN:?set OPS_TOKEN}"
: "${API_PUBLIC_URL:?set API_PUBLIC_URL}"

curl -fsS -H "Authorization: Bearer $OPS_TOKEN" "$API_PUBLIC_URL/api/v1/ops/status"
echo
