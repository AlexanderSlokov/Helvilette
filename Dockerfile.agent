FROM golang:1.25.6 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/agent ./cmd/agent

FROM ubuntu:24.04

# Avoid tzdata prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install Ansible and dependencies
RUN apt-get update && \
    apt-get install -y ansible ssh && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/agent .

# Environment variables
ENV OTHELA_URL=http://othela:8080

CMD ["./agent"]