# HomeDashboard

## Overview
HomeDashboard is an internal dashboard service for aggregating and visualizing home-related data (energy, weather, and tasks) via a web UI and HTTP API. It integrates with external APIs (e.g., Todoist) and provides image export functionality.

## Development

### Prerequisites
- Go (>=1.20)
- [bimg](https://github.com/h2non/bimg) and [libvips](https://libvips.github.io/libvips/) (for image processing)
- Docker (optional, for containerized builds)

### Running Locally
1. Clone the repository:
   ```sh
   git clone <internal-repo-url>
   cd homedashboard
   ```
2. Install Go dependencies:
   ```sh
   go mod tidy
   ```
3. Ensure `libvips` is installed on your system (see bimg/libvips docs for platform-specific instructions).
4. Run the service:
   ```sh
   go run main.go
   ```
5. Access the dashboard at [http://localhost:8080](http://localhost:8080) (default port).

## Docker Build & Run
1. Build the Docker image:
   ```sh
   docker build --platform=linux/amd64 -t homedashboard:latest .
   ```
2. Run the container:
   ```sh
   docker run -p 8080:8080 --env-file .env homedashboard:latest
   ```
   - Adjust ports and environment variables as needed.

---
**Note:** This project is for internal use only. Do not publish or distribute externally. 