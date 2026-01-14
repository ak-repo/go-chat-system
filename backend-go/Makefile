# ==============================
# Default target
# ==============================
.PHONY: help
help: ## Show available commands
	@echo "Available targets:" \
	&& grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	awk 'BEGIN {FS = ":.*?## "} {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

# ==============================
# Load config using yq (only if exists)
# ==============================
CONFIG_FILE := ./config/config.yaml

ifneq ("$(wildcard $(CONFIG_FILE))","")
DB_HOST       := $(shell yq e '.database.host' $(CONFIG_FILE))
DB_PORT       := $(shell yq e '.database.port' $(CONFIG_FILE))
DB_USER       := $(shell yq e '.database.user' $(CONFIG_FILE))
DB_PASSWORD   := $(shell yq e '.database.password' $(CONFIG_FILE))
DB_NAME       := $(shell yq e '.database.name' $(CONFIG_FILE))
DB_SSLMODE    := $(shell yq e '.database.sslmode' $(CONFIG_FILE))

REDIS_HOST    := $(shell yq e '.redis.host' $(CONFIG_FILE))
REDIS_PORT    := $(shell yq e '.redis.port' $(CONFIG_FILE))
REDIS_PASS    := $(shell yq e '.redis.password' $(CONFIG_FILE))

APP_PORT      := $(shell yq e '.server.port' $(CONFIG_FILE))
endif

DB_DSN := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

# ==============================
# Pre-flight checks
# ==============================
.PHONY: check
check: ## Check required tools and config
	@command -v yq >/dev/null 2>&1 || { echo "❌ yq not installed"; exit 1; }
	@command -v goose >/dev/null 2>&1 || { echo "❌ goose not installed"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "❌ go not installed"; exit 1; }
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ config/config.yaml not found"; exit 1; \
	fi
	@echo "✅ All checks passed"

# ==============================
# Build & Run
# ==============================
.PHONY: build
build: check ## Build Go binary
	go build -o bin/server ./cmd/server

.PHONY: run
run: check ## Run application locally
	go run ./cmd/server

# ==============================
# Database migrations (Goose)
# ==============================
.PHONY: migrate-create
migrate-create: check ## Create new migration
	@read -p "Migration name: " name; \
	goose -dir migrations create $$name sql

.PHONY: migrate-up
migrate-up: check ## Apply all migrations
	goose -dir migrations postgres "$(DB_DSN)" up

.PHONY: migrate-down
migrate-down: check ## Rollback last migration
	goose -dir migrations postgres "$(DB_DSN)" down

.PHONY: migrate-status
migrate-status: check ## Show migration status
	goose -dir migrations postgres "$(DB_DSN)" status

# ==============================
# Docker
# ==============================
.PHONY: docker-up
docker-up: check ## Start Postgres & Redis
	docker compose up -d

.PHONY: docker-down
docker-down: check ## Stop Docker services
	docker compose down

.PHONY: docker-clean
docker-clean: check ## Remove containers, volumes
	docker compose down -v
