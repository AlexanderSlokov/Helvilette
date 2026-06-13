# Helvilette Makefile
# ===================

.PHONY: all build test test-verbose test-cover cover-html clean run-othela run-agent up down logs seed e2e

# Default target
all: build

# Build binaries
build:
	go build -o bin/othela ./cmd/othela
	go build -o bin/agent ./cmd/agent

# Run all tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test ./... -v

# Run tests with coverage summary
test-cover:
	go test ./... -cover

# Generate coverage HTML report
cover-html:
	go test ./... -coverprofile=coverage.out
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
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f

seed:
	@echo "Seeding is now automatically handled by docker-compose via git-seeder container."

e2e:
	@export PATH=$$PATH:~/go/bin:/home/stella/sdk/go1.26.1/bin && ginkgo run ./tests/e2e/...
# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Tidy go modules
tidy:
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...