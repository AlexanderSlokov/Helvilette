FROM golang:1.25.6 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
# CGO_ENABLED=1 required for mattn/go-sqlite3 (same driver as k3s/kine).
# golang:1.25.6 image already has gcc installed.
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/othela ./cmd/othela

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=builder /app/othela .

# Expose the API port
EXPOSE 8080

CMD ["./othela"]