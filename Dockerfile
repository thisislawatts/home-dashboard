# ---- Build stage ----
FROM golang:1.24 AS builder

WORKDIR /app

# Install build dependencies and libvips-dev for CGO
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        build-essential \
        libvips-dev && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with CGO enabled and strip debug info
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o homedashboard main.go

# ---- Final stage ----
FROM debian:bookworm-slim


WORKDIR /app

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libvips \
        chromium \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/homedashboard .
COPY src/ ./src/

EXPOSE 8080

ENV CHROME_PATH=/usr/bin/chromium

ENTRYPOINT ["./homedashboard"] 