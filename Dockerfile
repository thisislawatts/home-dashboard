# ---- Build stage ----
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git (for go mod) and build dependencies
RUN apk add --no-cache git

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -o homedashboard main.go

# ---- Final stage ----
FROM alpine:3.19

WORKDIR /app

# Install Chromium and required dependencies
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ttf-freefont \
    ca-certificates

# Copy the Go binary and static files
COPY --from=builder /app/homedashboard .
COPY src/ ./src/
COPY dashboard.html ./
COPY dist/ ./dist/
COPY wrangler.jsonc ./

# Expose the default port
EXPOSE 8080

# Set environment variables for chromedp to use Chromium
ENV CHROME_PATH=/usr/bin/chromium-browser

# Run the Go binary
ENTRYPOINT ["./homedashboard"] 