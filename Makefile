.PHONY: build run test test-short test-integration sqlc sqlc-check db-up db-down

TEST_DATABASE_URL ?= postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable

db-up:
	docker compose up -d --wait postgres

db-down:
	docker compose down

build:
	go build -o ./bin/server ./cmd/server

run:
	go run ./cmd/server

test-short:
	go test ./... -short -race -count=1

test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1

test: test-integration

sqlc:
	sqlc generate

sqlc-check:
	sqlc generate
	git diff --exit-code -- internal/db/sqlc/
