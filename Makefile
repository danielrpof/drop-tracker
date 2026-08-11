.PHONY: build run test test-short test-integration sqlc sqlc-check sqlc-version-check db-up db-down hooks web

TEST_DATABASE_URL ?= postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable

# Override on boxes where the interpreter is `python3` instead of `python`.
PYTHON ?= python

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

# Builds the SPA (web/) and replaces the committed
# internal/webassets/build/client/ tree with the fresh output, so
# go:embed picks up the latest build. --frozen-lockfile refuses to
# silently resolve a different dependency version than pnpm-lock.yaml
# pins (T-06-SC). Phase 7's multi-stage Docker image rebuilds this output
# in its own Node build stage rather than trusting the tree committed
# here -- the commit exists solely so `go build`, `go vet` and
# `go test ./...` all work on a clone that has never run the Node
# toolchain (06-RESEARCH.md).
web:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm run build
	rm -rf internal/webassets/build/client
	mkdir -p internal/webassets/build
	cp -r web/build/client internal/webassets/build/client

# Installs the pre-commit framework and the git hook shim it defines
# (see .pre-commit-config.yaml). pre-commit builds the pinned gitleaks
# binary itself from that config -- no separate gitleaks install needed.
hooks:
	$(PYTHON) -m pip install --user --upgrade pre-commit
	$(PYTHON) -m pre_commit install --install-hooks
