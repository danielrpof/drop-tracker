.PHONY: build run test test-short test-integration coverage-report coverage-gate sqlc sqlc-check sqlc-version-check db-up db-down hooks web

# Must stay in lockstep with docker-compose.yml's published port -- see the
# comment there. Pointing this at a port another project already holds does
# not fail loudly; it silently runs migrations and writes test rows into that
# other project's database. Was briefly repointed at 5433 during a stray
# leftover Postgres container's collision with the host's default 5432; that
# container has since been shut down, so this is back on 5432 permanently.
TEST_DATABASE_URL ?= postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable

# Override on boxes where the interpreter is `python3` instead of `python`.
PYTHON ?= python

# Pinned exactly to the version verified in 01-RESEARCH.md's Package
# Legitimacy Audit (T-01-13) -- a silent sqlc upgrade must fail generation
# rather than regenerate committed code with an unaudited toolchain version.
SQLC_VERSION := v1.31.1

# The generated sqlc package is filtered out because `sqlc-check` (below)
# already proves it matches the schema -- testing it directly would only
# re-test sqlc itself, not this codebase (09-CONTEXT.md D-04). Every other
# package stays in, including cmd/server, which has no test files of its own
# and would silently never enter the coverage profile under Go's default
# self-package-only instrumentation (09-CONTEXT.md D-05). Deferred assignment
# (`=`, not `:=`) so the `go list` subprocess only runs when a target that
# actually references this variable is built. The exclusion pattern is
# anchored (`(^|/)...$$`) rather than a bare substring match -- an unanchored
# match would also drop a future package such as internal/db/sqlcgen or
# internal/db/sqlc_helpers from the coverage profile, silently changing the
# percentage the CICD-11 gate enforces (09-REVIEW.md WR-02). cmd/coverage-report
# is dropped for a second reason (15-CONTEXT.md D-07): a CI helper that reports
# the backend coverage number must not sit in the denominator of the metric it
# reports. The doubled `$$` is required: a single `$` is consumed by make's own
# variable expansion before the shell ever sees it, silently turning the anchor
# into an empty string.
COVER_PKGS = $(shell go list ./... | grep -vE '(^|/)(internal/db/sqlc|cmd/coverage-report)$$' | paste -sd, -)

# CICD-11: 80% is the required floor for aggregate backend coverage, not a
# tunable -- `?=` only exists so this can be overridden on the command line
# for pass/fail smoke-testing (e.g. `make coverage-gate
# COVERAGE_THRESHOLD_BACKEND=0`), never to permanently lower it in this file.
COVERAGE_THRESHOLD_BACKEND ?= 80

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

# Runs every package binary concurrently, at the toolchain's own default
# package-level parallelism setting -- no invocation flag pins it down to
# one at a time anymore. That pin used to be required here: internal/db's
# migrate-from-scratch test used to `DROP SCHEMA public CASCADE` against the
# shared fixture on a bare connection, outside golang-migrate's own
# advisory-lock serialisation, and internal/notifier's NotifyPending
# queried/marked the shared events table via a deliberately global,
# unfiltered query (D-06) -- both let one package's DB-backed tests corrupt
# or pollute another's when run concurrently. Both root causes are fixed at
# the source rather than masked: see internal/db/migrate_test.go's dedicated
# migrate_scratch schema and internal/notifier/notifier_test.go's
# testutil.NewIsolatedTestPool. Verified stable across 5 separate
# consecutive full-suite runs at default parallelism
# (.planning/todos/completed/2026-08-11-fix-flaky-tests-under-parallel-go-test.md).
test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 \
		-coverprofile=coverage.out -coverpkg=$(COVER_PKGS)

test: test-integration

# D-17: the single place the backend coverage total is measured. coverage-gate
# consumes this same tool and mode, so the gate and the PR coverage comment
# cannot disagree by construction. Prints only the bare 2-decimal number to
# stdout; a missing profile is a loud stderr diagnostic and a non-zero exit.
coverage-report:
	@if [ ! -s coverage.out ]; then \
		echo "coverage.out not found or empty -- run 'make test-integration' first" >&2; \
		exit 1; \
	fi
	@go run ./cmd/coverage-report --mode=total --profile=coverage.out

# Hand-rolled coverage gate (09-CONTEXT.md D-01) -- no prerequisites, so CI
# and a developer can run it immediately after test-integration without
# re-running the suite. Log-only (D-03): the two echoes below are the entire
# report surface, no HTML report and no artifact upload. The measured number
# comes from `coverage-report` (the cmd/coverage-report tool, D-17) so the gate
# and the PR coverage comment share one algorithm; the 80 literal stays here.
coverage-gate:
	@if [ ! -s coverage.out ]; then \
		echo "coverage.out not found or empty -- run 'make test-integration' first" >&2; \
		exit 1; \
	fi
	@coverage=$$(go run ./cmd/coverage-report --mode=total --profile=coverage.out); \
	if [ -z "$$coverage" ]; then \
		echo "failed to parse aggregate coverage total from coverage.out" >&2; \
		exit 1; \
	fi; \
	echo "Backend coverage: $${coverage}% (required: $(COVERAGE_THRESHOLD_BACKEND)%)"; \
	if awk -v cov="$$coverage" -v thresh="$(COVERAGE_THRESHOLD_BACKEND)" 'BEGIN { exit !(cov + 0 >= thresh + 0) }'; then \
		echo "PASS"; \
	else \
		echo "FAIL: $${coverage}% is below the required $(COVERAGE_THRESHOLD_BACKEND)% threshold" >&2; \
		exit 1; \
	fi

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
