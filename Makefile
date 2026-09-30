COMPOSE := docker compose -f local/compose.yml

.PHONY: generate migration fmt lint build run up down prod prod-logs

# Ent client code and the DDL snapshot (internal/ent/migrate/schema.sql).
generate:
	go generate ./internal/ent/generate.go
	go generate ./internal/ent/generate_ddl_linux.go

# Atlas migration from the schema snapshot. Run only when the schema change is final.
migration:
	cd internal/ent && go generate ./generate_atlas.go

fmt:
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...

build:
	go build -o bin/cameo .

run:
	CONFIG_DIR=./local/config MIGRATIONS_DIR=internal/ent/migrate/migrations LISTEN_ADDRESS=:18080 go run .

up:
	$(COMPOSE) up -d database s3 bucket-init

down:
	$(COMPOSE) down

# Production deploy on the host, run from the repository root.
prod:
	VERSION=$$(git rev-parse --short HEAD) docker compose up -d --build --remove-orphans

prod-logs:
	docker compose logs -f --tail=200 cameo
