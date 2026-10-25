#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
cd "$root/backend"

exec go test -tags=integration -count=1 -timeout 15m ./internal/deployments -run TestExampleNext
