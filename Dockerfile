# ---- Build stage ----
FROM golang:1.24 AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with CGO enabled and strip debug info
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o homedashboard main.go

# ---- Final stage ----
FROM debian:bookworm-slim

WORKDIR /app

# Install runtime dependencies (minimal chromium)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        chromium \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/homedashboard .
COPY src/ ./src/

EXPOSE 8080

ENV CHROME_PATH=/usr/bin/chromium

ENTRYPOINT ["./homedashboard"] 