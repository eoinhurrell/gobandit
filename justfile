# GoBandit task runner. Run `just` (no args) to list all recipes.

# ---- config -----------------------------------------------------------------
go       := "go"
bin_dir  := "bin"
pg       := "postgres"          # docker compose service name
db_url   := "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable"

# default: show available recipes (just lists them automatically)
default:
    @just --list

# ---- dependencies -----------------------------------------------------------

# Install the developer tools the other recipes rely on (templ, golangci-lint).
tools:
    {{go}} install github.com/a-h/templ/cmd/templ@latest
    {{go}} install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Tidy module files (go.mod / go.sum).
tidy:
    {{go}} mod tidy

# ---- code generation --------------------------------------------------------

# Regenerate Go from templ sources (templates/*.templ) and gofmt the output.
# templ artifacts are committed; go.mod is pinned to match the installed CLI.
generate:
    templ generate
    gofmt -w templates

# Format templ sources in place.
templ-fmt:
    templ fmt templates

# ---- building ---------------------------------------------------------------

# Compile everything without producing binaries (matches CI).
build:
    {{go}} build -v ./...

# Build all three server binaries into ./bin.
bin:
    {{go}} build -o {{bin_dir}}/gobandit         .
    {{go}} build -o {{bin_dir}}/assignment-api   ./cmd/assignment-api
    {{go}} build -o {{bin_dir}}/control-plane    ./cmd/control-plane

# ---- running ----------------------------------------------------------------

# Run the main server (the README quickstart). Needs Postgres on localhost:5432.
run:
    {{go}} run .

# Run the assignment API server.
run-assignment-api:
    {{go}} run ./cmd/assignment-api

# Run the control-plane web UI server.
run-control-plane:
    {{go}} run ./cmd/control-plane

# ---- testing ----------------------------------------------------------------

# Run the unit + integration test suite.
test:
    {{go}} test ./...

# Run the suite verbosely.
test-verbose:
    {{go}} test -v ./...

# Run tests with the race detector.
test-race:
    {{go}} test -race ./...

# Run the Postgres-backed migration tests (brings up docker compose postgres first).
test-integration: db-up
    {{go}} test ./migrations/...

# Run benchmarks.
bench:
    {{go}} test -bench=. -benchmem ./...

# Generate a coverage profile and HTML report at bin/coverage.html.
cover:
    {{go}} test -coverprofile={{bin_dir}}/coverage.out ./...
    {{go}} tool cover -html={{bin_dir}}/coverage.out -o {{bin_dir}}/coverage.html

# ---- static checks ----------------------------------------------------------

# go vet.
vet:
    {{go}} vet ./...

# Format all Go sources in place.
fmt:
    {{go}} fmt ./...

# Fail if any Go source is not gofmt-clean.
fmt-check:
    @test -z "$({{go}}fmt -l .)" || { echo "gofmt would change these files:"; {{go}}fmt -l .; exit 1; }

# Run golangci-lint.
lint:
    golangci-lint run

# Full local gate: format check, vet, lint, build, test.
check: fmt-check vet lint build test

# Mirror the GitHub Actions CI pipeline exactly.
ci: build test

# ---- database ---------------------------------------------------------------

# Start PostgreSQL (init.sql is auto-applied on a fresh volume).
db-up:
    docker compose up -d {{pg}}

# Stop PostgreSQL (keeps data volume).
db-down:
    docker compose down

# Stop PostgreSQL and delete its data volume (re-applies init.sql on next db-up).
db-reset:
    docker compose down -v

# Tail PostgreSQL logs.
db-logs:
    docker compose logs -f {{pg}}

# Open a psql shell against the running container.
db-shell:
    docker compose exec {{pg}} psql -U postgres

# Apply migrations/*.up.sql to the running Postgres (best-effort, skips already-applied statements).
migrate:
    #!/usr/bin/env bash
    set -euo pipefail
    for f in migrations/*.up.sql; do
        echo "==> applying $f"
        docker compose exec -T {{pg}} psql -U postgres -v ON_ERROR_STOP=0 < "$f"
    done

# ---- housekeeping -----------------------------------------------------------

# Remove build artifacts and coverage output.
clean:
    rm -rf {{bin_dir}}
