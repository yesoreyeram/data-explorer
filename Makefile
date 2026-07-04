.PHONY: help dev-backend dev-frontend build-backend build-frontend test lint up down logs

help:
	@echo "make dev-backend      run the Go API locally (needs DATABASE_URL)"
	@echo "make dev-frontend     run the Vite dev server"
	@echo "make test             run backend tests"
	@echo "make lint             run backend vet + frontend lint/typecheck"
	@echo "make build-backend    build the backend binary to backend/bin/server"
	@echo "make build-frontend   build the frontend static bundle"
	@echo "make up               docker compose up --build (see deploy/.env.example)"
	@echo "make down             docker compose down"
	@echo "make logs             tail docker compose logs"

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

test:
	cd backend && go test ./...

lint:
	cd backend && go vet ./...
	cd frontend && npx tsc -b && npm run lint

build-backend:
	cd backend && go build -o bin/server ./cmd/server

build-frontend:
	cd frontend && npm run build

up:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f
