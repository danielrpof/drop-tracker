.PHONY: build run test test-short test-integration sqlc sqlc-check sqlc-version-check db-up db-down

TEST_DATABASE_URL ?= postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable

# Pinned exactly to the version verified in 01-RESEARCH.md's Package
# Legitimacy Audit (T-01-13) -- a silent sqlc upgrade must fail generation
# rather than regenerate committed code with an unaudited toolchain version.
SQLC_VERSION := v1.31.1

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

sqlc-version-check:
	@actual=$$(sqlc version); \
	if [ "$$actual" != "$(SQLC_VERSION)" ]; then \
		echo "sqlc version mismatch: want $(SQLC_VERSION), got $$actual" >&2; \
		exit 1; \
	fi

sqlc: sqlc-version-check
	sqlc generate

sqlc-check: sqlc-version-check
	sqlc generate
	git diff --exit-code -- internal/db/sqlc/
