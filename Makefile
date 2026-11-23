.DEFAULT_GOAL := help

.PHONY: help dev-infra stop-infra observability-up observability-down api worker worker-once web test lint format coverage e2e prod-up prod-down

help:
	@echo "dev-infra           postgres, redis, traefik"
	@echo "stop-infra          stop those containers"
	@echo "observability-up    optional prometheus + grafana profile"
	@echo "observability-down  stop observability profile"
	@echo "api                 go api on :8080, applies migrations, publishes queued jobs"
	@echo "worker              queue worker. consumes redis and builds deployments"
	@echo "worker-once         one repo, no queue: REPO=https://github.com/owner/name make worker-once"
	@echo "web                 next.js on :3000"
	@echo "test                go tests and frontend typecheck + vitest"
	@echo "coverage            go tests with coverage summary"
	@echo "e2e                 Traefik + example node-demo routed URL"
	@echo "prod-up             build/start infrastructure/production stack (needs .env)"
	@echo "prod-down           stop production stack"
	@echo "lint                go vet and eslint"
	@echo "format              gofmt and eslint --fix"

dev-infra:
	docker compose up -d

stop-infra:
	docker compose down

observability-up:
	docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability up -d

observability-down:
	docker compose -f docker-compose.yml -f docker-compose.observability.yml --profile observability down

api:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker

worker-once:
	cd backend && go run ./cmd/worker --repo "$(REPO)" --branch "$(BRANCH)"

web:
	cd apps/web && pnpm dev

test:
	cd backend && go test ./...
	cd apps/web && pnpm typecheck
	cd apps/web && pnpm test

coverage:
	cd backend && go test -count=1 -coverprofile=coverage.out ./...
	cd backend && go tool cover -func=coverage.out | tail -n 1

e2e:
	infrastructure/scripts/e2e-deploy.sh

prod-up:
	cd infrastructure/production && docker compose --env-file .env up -d --build

prod-down:
	cd infrastructure/production && docker compose --env-file .env down

lint:
	cd backend && go vet ./...
	cd apps/web && pnpm lint

format:
	gofmt -w backend
	cd apps/web && pnpm exec eslint --fix .
