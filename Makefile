# Helvilette Makefile
# ===================

.PHONY: all build test test-verbose test-cover cover-html clean clean-e2e \
        run-othela run-agent run-agent-prod up down logs seed e2e tidy \
        fmt fmt-check lint

# Go packages that make up the shippable code. Deliberately excludes
# ./tests/... : the e2e suite is driven by ginkgo (make e2e), and a plain
# `./...` also trips over container-created state under tests/e2e/data
# (see clean-e2e below). Keep unit-test targets pointed here.
GO_PKGS := ./cmd/... ./pkg/...

# The repo has no default compose file, so every docker compose call must
# name this one explicitly.
COMPOSE_FILE := docker-compose.e2e.yaml
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

# Run Othela Control Plane
run-othela:
	go run ./cmd/othela

# Run Agent (dev mode with human-readable logs)
run-agent:
	HELVILETTE_DEV=1 go run ./cmd/agent

# Run Agent (production mode with JSON logs)
run-agent-prod:
	go run ./cmd/agent

# Docker Compose Targets
up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f

seed:
	@echo "Seeding is now automatically handled by docker-compose via git-seeder container."

e2e:
	@export PATH=$$PATH:~/go/bin:/home/stella/sdk/go1.26.1/bin && ginkgo run ./tests/e2e/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Remove container-created state left by older stacks.
#
# Since ADR-0003 Othela writes its state to a named volume, so a current stack
# creates nothing here. Stacks from before that change left
# tests/e2e/data/playbooks/server owned by root:root mode 750, which the host
# user cannot read, so `go vet ./...` and `go list ./...` failed with
# "permission denied" before examining any code. Deleting it needs root, so
# borrow a container's root rather than asking for sudo. Harmless to run when
# the directories are already gone.
clean-e2e:
	$(COMPOSE) down -v --remove-orphans 2>/dev/null || true
	@mkdir -p data
	docker run --rm \
		-v "$(CURDIR)/tests/e2e/data:/e2e-data" \
		-v "$(CURDIR)/data:/agent-data" \
		alpine:3.19 \
		rm -rf /e2e-data/playbooks/server /agent-data/agent-1 /agent-data/agent-2

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
