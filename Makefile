# Helvilette Makefile
# ===================

.PHONY: all build test test-verbose test-cover cover-html clean clean-e2e \
        run-othela run-othela-prod run-agent run-agent-prod up down logs journal \
        images e2e tidy fmt fmt-check lint

# Go packages that make up the shippable code. Deliberately excludes
# ./tests/... : the e2e suite is driven by ginkgo (make e2e) and needs a Docker
# daemon, so it must not run as part of a plain unit-test invocation.
GO_PKGS := ./cmd/... ./pkg/...

# The repo has no default compose file, so every docker compose call must
# name this one explicitly. This file is the only definition of the e2e stack:
# `make up` and `make e2e` both run it. See ADR-0007.
COMPOSE_FILE := e2e.compose.yml
COMPOSE := docker compose -f $(COMPOSE_FILE)

# Default target
all: build

# Build binaries
build:
	CGO_ENABLED=1 go build -o bin/othela ./cmd/othela
	CGO_ENABLED=1 go build -o bin/agent ./cmd/agent

# Run unit tests (fast). End-to-end lives in `make e2e`.
test:
	go test $(GO_PKGS)

# Run tests with verbose output
test-verbose:
	go test $(GO_PKGS) -v

# Run tests with coverage summary
test-cover:
	go test $(GO_PKGS) -cover

# Generate coverage HTML report
cover-html:
	go test $(GO_PKGS) -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run Othela Control Plane (dev mode with human-readable logs)
run-othela:
	HELVILETTE_DEV=1 go run ./cmd/othela

# Run Othela Control Plane (production mode with JSON logs)
run-othela-prod:
	go run ./cmd/othela

# Run Agent (dev mode with human-readable logs)
run-agent:
	HELVILETTE_DEV=1 go run ./cmd/agent

# Run Agent (production mode with JSON logs)
run-agent-prod:
	go run ./cmd/agent

# Docker Compose Targets
#
# --wait blocks until every service reports healthy, using the healthcheck blocks
# in the compose file, so a stack that came up broken fails here rather than in
# whatever you run next.
up:
	$(COMPOSE) up -d --build --wait --wait-timeout 600

down:
	$(COMPOSE) down -v --remove-orphans

logs:
	$(COMPOSE) logs -f

# Per-unit output lives in each node's journal, not in its container log, because
# the nodes run systemd. `make logs` shows the boot transcript; this shows
# Helvilette. NODE defaults to the control plane.
NODE ?= othela
journal:
	$(COMPOSE) exec $(NODE) journalctl --no-pager -f -t 'helvilette*'

# Builds the distributable images. Nothing else in this repo consumes them, so
# without this target they would drift out of buildability unnoticed, which is
# how the Vagrant environment died. CI runs it too. See ADR-0007.
images:
	docker build -f build/othela/Dockerfile -t helvilette-othela:dev .
	docker build -f build/agent/Dockerfile -t helvilette-agent:dev .

# Drives $(COMPOSE_FILE) itself: brings the stack up, asserts, tears it down.
e2e:
	go run github.com/onsi/ginkgo/v2/ginkgo run ./tests/e2e/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Remove anything a stack left behind.
#
# Since ADR-0007 the e2e stack writes nothing into the source tree: fixtures are
# baked into the git server image at build time and state lives in container
# filesystems and named volumes. So this is `compose down -v` and little else.
# The previous version had to borrow a container's root to delete
# tests/e2e/data/playbooks/server, created root:root mode 750 by an older stack,
# which made `go vet ./...` fail before it examined any code.
clean-e2e:
	$(COMPOSE) down -v --remove-orphans 2>/dev/null || true
	docker image rm -f helvilette-gitserver:e2e helvilette-othela-node:e2e helvilette-agent-node:e2e 2>/dev/null || true

# Tidy go modules
tidy:
	go mod tidy

# Format code
fmt:
	gofmt -w ./cmd ./pkg

# Verify formatting without rewriting files. Used by CI.
fmt-check:
	@unformatted=$$(gofmt -l ./cmd ./pkg); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted. Run 'make fmt':"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "gofmt: all files formatted"

# Lint (requires golangci-lint)
lint:
	golangci-lint run $(GO_PKGS)
