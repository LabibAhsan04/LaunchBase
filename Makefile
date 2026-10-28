.DEFAULT_GOAL := help

.PHONY: help dev-infra stop-infra api worker web test lint format

help:
	@echo "dev-infra    postgres, redis, traefik"
	@echo "stop-infra   stop those containers"
	@echo "api          go api on :8080, applies migrations"
	@echo "worker       deploy a github repo: REPO=https://github.com/owner/name make worker"
	@echo "web          next.js on :3000"
	@echo "test         go tests and frontend typecheck"
	@echo "lint         go vet and eslint"
	@echo "format       gofmt and eslint --fix"

dev-infra:
	docker compose up -d

stop-infra:
	docker compose down

api:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker --repo "$(REPO)" --branch "$(BRANCH)"

web:
	cd apps/web && pnpm dev

test:
	cd backend && go test ./...
	cd apps/web && pnpm typecheck

lint:
	cd backend && go vet ./...
	cd apps/web && pnpm lint

format:
	gofmt -w backend
	cd apps/web && pnpm exec eslint --fix .
