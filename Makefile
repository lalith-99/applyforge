.PHONY: dev test lint migrate seed build docker-up docker-down fmt

dev: docker-up
	@echo "Postgres + api + ai-worker running via docker-compose."
	@echo "Run the web app separately: cd apps/web && pnpm dev"

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

build:
	cd apps/api && go build ./...
	cd apps/web && pnpm install && pnpm build
	cd apps/ai-worker && ./.venv/bin/pip install -q -r requirements.txt

fmt:
	cd apps/api && gofmt -l .
	cd apps/ai-worker && ./.venv/bin/ruff format .

lint:
	cd apps/api && go vet ./...
	cd apps/ai-worker && ./.venv/bin/ruff check .
	cd apps/web && pnpm lint

test:
	cd apps/api && go test ./...
	cd apps/ai-worker && ./.venv/bin/python -m pytest -q

migrate:
	cd apps/api && goose -dir internal/database/migrations postgres "$${DATABASE_URL:-postgres://applyforge:applyforge@localhost:5433/applyforge?sslmode=disable}" up

seed:
	@echo "No seed data yet — introduced in a later phase."
