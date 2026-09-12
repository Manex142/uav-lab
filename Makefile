# ==============================================================================
# 🛸 UAV Lab - Build & Automation Makefile
# ==============================================================================

SHELL := /bin/bash
BIN_DIR := bin
DOCKER_COMPOSE := docker compose -f docker/docker-compose.yml

MIGRATIONS_DIR := internal/database/migrations

.PHONY: all help dev sim swarm db-up db-down db-logs test bench build clean tidy migration

all: test build

## help: Print this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'

## dev: Run the high-throughput telemetry gateway
dev:
	go run ./cmd/gateway

## sim: Run single-drone synthetic telemetry simulator (100 Hz)
sim:
	go run ./cmd/simulator -drones=1 -frequency=100

## swarm: Run concurrent drone swarm simulator (e.g. make swarm drones=50 hz=100)
swarm:
	go run ./cmd/simulator -drones=$(if $(drones),$(drones),20) -frequency=$(if $(hz),$(hz),100)

## db-up: Start TimescaleDB container in background
db-up:
	$(DOCKER_COMPOSE) up -d timescaledb

## db-down: Stop all project Docker containers
db-down:
	$(DOCKER_COMPOSE) down

## db-logs: Follow TimescaleDB container logs
db-logs:
	$(DOCKER_COMPOSE) logs -f timescaledb

## migration: Create a new timestamped SQL migration (e.g. make migration name=add_index)
migration:
	@if [ -z "$(name)" ]; then \
		echo "❌ Error: Specify migration name (e.g. make migration name=add_index)"; \
		exit 1; \
	fi
	@TIMESTAMP=$$(date -u +'%Y%m%d%H%M%S'); \
	FILE="$(MIGRATIONS_DIR)/$${TIMESTAMP}_$(name).sql"; \
	mkdir -p $(MIGRATIONS_DIR); \
	printf -- "-- +goose Up\n\n\n-- +goose Down\n" > $$FILE; \
	echo "✅ Created new migration: $$FILE"

## test: Run unit & integration tests with data race detector
test:
	go test -v -race ./...

## bench: Run Go performance benchmarks with memory allocations
bench:
	go test -v -bench=. -benchmem ./...

## tidy: Format code and tidy Go module dependencies
tidy:
	go fmt ./...
	go mod tidy

## build: Compile native binaries for gateway and simulator into bin/
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/gateway ./cmd/gateway
	go build -o $(BIN_DIR)/simulator ./cmd/simulator
	@echo "✅ Binaries compiled to $(BIN_DIR)/"

## clean: Remove compiled binaries and build artifacts
clean:
	rm -rf $(BIN_DIR)
