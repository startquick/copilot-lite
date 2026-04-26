.PHONY: build run test clean dev smoke docker-up docker-down docker-logs docker-ps docker-restart docker-shell

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Go build
build:
	go build $(LDFLAGS) -o bin/copilotpi ./cmd/copilotpi

run: build
	./bin/copilotpi

dev:
	go run $(LDFLAGS) ./cmd/copilotpi

test:
	go test -race -v ./...

smoke:
	@if [ -z "$(BASE_URL)" ]; then echo "BASE_URL is required, example: make smoke BASE_URL=http://127.0.0.1:8080 APP_KEY=... API_KEY=..."; exit 1; fi
	@if [ -z "$(APP_KEY)" ]; then echo "APP_KEY is required"; exit 1; fi
	@if [ -z "$(API_KEY)" ]; then echo "API_KEY is required"; exit 1; fi
	@echo "== /health =="
	@curl -fsS "$(BASE_URL)/health"
	@echo ""
	@echo "== /admin/verify =="
	@curl -fsS "$(BASE_URL)/admin/verify" -H "Authorization: Bearer $(APP_KEY)"
	@echo ""
	@echo "== /v1/models =="
	@curl -fsS "$(BASE_URL)/v1/models" -H "Authorization: Bearer $(API_KEY)"
	@echo ""
	@echo "== /admin/tokens?page_size=10 =="
	@curl -fsS "$(BASE_URL)/admin/tokens?page_size=10" -H "Authorization: Bearer $(APP_KEY)"
	@echo ""

clean:
	rm -rf bin/ data/

# ─── Docker Local (menggunakan .env → COMPOSE_FILE) ──────────────────────────
# Pastikan sudah ada .env dengan COMPOSE_FILE=docker-compose.yml:docker-compose.local.yml
# Lihat docs/deployment-checklist-lokal.md untuk setup awal.

docker-up:
	docker compose build --no-cache copilotpi
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f copilotpi

docker-ps:
	docker compose ps

docker-restart:
	docker compose restart copilotpi

docker-shell:
	docker compose exec copilotpi /bin/sh
