# portless-docker Setup Guide

## Prerequisites

- **Go 1.22+** — [Download](https://go.dev/dl/)
- **Docker** with `docker compose` v2 — [Install Docker](https://docs.docker.com/get-docker/)

## Build from Source

```bash
# Clone the repository
git clone https://github.com/DoskoiYuta/portless-docker.git
cd portless-docker

# Install dependencies
go mod tidy

# Build
make build
# Binary output: dist/portless-docker
```

## Install

```bash
# Option 1: Copy to PATH
sudo cp dist/portless-docker /usr/local/bin/

# Option 2: go install
go install github.com/DoskoiYuta/portless-docker/cmd/portless-docker@latest
```

## Quick Start

Navigate to a project directory with a `docker-compose.yml`:

```bash
cd ~/my-project

# Start services (replaces "docker compose up")
portless-docker up

# Access services via .localhost subdomains:
#   http://frontend.localhost:1355
#   http://api.localhost:1355
```

## Usage

### Start services (foreground)

```bash
portless-docker up
# Ctrl+C to stop — cleanup is automatic
```

### Start services (background)

```bash
portless-docker up -d
```

### List active routes

```bash
portless-docker ls
```

### Run commands against services

```bash
portless-docker exec api bash
portless-docker run api rails db:migrate
portless-docker logs -f frontend
```

### Stop services

```bash
portless-docker down
```

### Stop everything (all projects)

```bash
portless-docker stop --all
```

### Custom options

```bash
# Use a different proxy port
portless-docker -p 8888 up

# Specify compose file
portless-docker -f compose.prod.yml up

# Ignore specific services
portless-docker --ignore redis,postgres up -d
```

## How It Works

1. Parses `docker-compose.yml` to detect services with port mappings
2. Assigns dynamic host ports (range 40000–49999) to avoid conflicts
3. Generates a temporary override file in `/tmp/portless-docker-XXXX/`
4. Starts a reverse proxy on port 1355 that routes `<service>.localhost` subdomains
5. Runs `docker compose` with both the original and override compose files
6. Cleans up automatically on exit

### `.localhost` DNS

Modern browsers resolve `*.localhost` to `127.0.0.1` per [RFC 6761](https://datatracker.ietf.org/doc/html/rfc6761). No `/etc/hosts` changes or DNS configuration needed.

## State Files

State is stored in `~/.portless-docker/`:

| File | Purpose |
|------|---------|
| `state.json` | Active routes registry |
| `proxy.pid` | Proxy daemon PID |
| `proxy.port` | Proxy listen port |
| `proxy.log` | Proxy daemon logs |

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make build` | Build for current platform |
| `make build-all` | Cross-compile for macOS/Linux (amd64/arm64) |
| `make install` | Build and copy to `$GOPATH/bin` |
| `make test` | Run tests |
| `make test-verbose` | Run tests with verbose output |
| `make test-cover` | Run tests with coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code |
| `make vet` | Run go vet |
| `make tidy` | Tidy dependencies |
| `make clean` | Remove build artifacts |

## Troubleshooting

### Proxy fails to start

Check the proxy log:

```bash
cat ~/.portless-docker/proxy.log
```

### Port 1355 is already in use

Use a different port:

```bash
portless-docker -p 9999 up
```

### Stale state after crash

Stop everything and clean up:

```bash
portless-docker stop --all
```

### `.localhost` not resolving in browser

Most modern browsers support `.localhost` natively. If not, add entries to `/etc/hosts`:

```
127.0.0.1  frontend.localhost
127.0.0.1  api.localhost
```
