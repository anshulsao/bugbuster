# BugBuster

> Practice debugging production incidents in safe Docker environments.

BugBuster spins up a realistic microservices stack (Node.js, Python, Java, Go, nginx) with full observability (Grafana, Jaeger, Prometheus, Loki), injects a real bug, and challenges you to find and fix it.

```
  ____              ____            _
 | __ ) _   _  __ _| __ ) _   _ ___| |_ ___ _ __
 |  _ \| | | |/ _` |  _ \| | | / __| __/ _ \ '__|
 | |_) | |_| | (_| | |_) | |_| \__ \ ||  __/ |
 |____/ \__,_|\__, |____/ \__,_|___/\__\___|_|
              |___/
```

## Install

### Option 1: Download Binary (Recommended)

Grab the latest release for your platform:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/anshulsao/bugbuster/releases/latest/download/bugbuster-darwin-arm64 -o bugbuster
chmod +x bugbuster
sudo mv bugbuster /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/anshulsao/bugbuster/releases/latest/download/bugbuster-darwin-amd64 -o bugbuster
chmod +x bugbuster
sudo mv bugbuster /usr/local/bin/

# Linux (x86_64)
curl -L https://github.com/anshulsao/bugbuster/releases/latest/download/bugbuster-linux-amd64 -o bugbuster
chmod +x bugbuster
sudo mv bugbuster /usr/local/bin/

# Linux (ARM64)
curl -L https://github.com/anshulsao/bugbuster/releases/latest/download/bugbuster-linux-arm64 -o bugbuster
chmod +x bugbuster
sudo mv bugbuster /usr/local/bin/
```

### Option 2: Build from Source

Requires **Go 1.22+**.

```bash
git clone https://github.com/anshulsao/bugbuster.git
cd bugbuster
go build -o bugbuster ./cmd/bugbuster
sudo mv bugbuster /usr/local/bin/
```

### Prerequisites

BugBuster runs services in Docker. You need:

- **Docker** (v20+) with **Docker Compose** v2
- ~4 GB free RAM (for all containers)

Verify Docker is running:

```bash
docker compose version
# Docker Compose version v2.x.x
```

## Quick Start

```bash
# Launch the interactive TUI
bugbuster
```

This opens a terminal UI where you can browse scenarios, start environments, test APIs, get hints, and submit your root cause analysis — all from one screen.

### CLI Mode (no TUI)

```bash
# List available scenarios
bugbuster list

# Start a scenario
bugbuster start ghost-latency

# Get a hint (costs points)
bugbuster hint

# Check session status
bugbuster status

# Submit your RCA
bugbuster submit

# Tear down when done
bugbuster stop

# View your scores
bugbuster leaderboard
```

## How It Works

```
  You run bugbuster start <scenario>
           │
           ▼
  ┌─────────────────────────────┐
  │  Docker Compose brings up:  │
  │                             │
  │  nginx ─► order (Node.js)   │
  │       ─► catalog (Python)   │
  │       ─► payment (Java)     │
  │       ─► notifier (Go)      │
  │                             │
  │  + Redis, RabbitMQ, DBs     │
  │  + Grafana, Jaeger, Prom    │
  │                             │
  │  A bug is injected via      │
  │  compose override + env vars│
  └─────────────────────────────┘
           │
           ▼
  You investigate using real
  observability tools:
    Grafana   → localhost:3000
    Jaeger    → localhost:16686
    Prometheus→ localhost:9091
           │
           ▼
  bugbuster submit
    → checks your RCA category
    → runs validation scripts
    → scores your performance
```

## Observability Stack

Once a scenario is running, these tools are available:

| Tool | URL | Credentials |
|---|---|---|
| Grafana | http://localhost:3000 | admin / bugbuster |
| Jaeger | http://localhost:16686 | — |
| Prometheus | http://localhost:9091 | — |
| Loki (via Grafana) | http://localhost:3000 | — |
| RabbitMQ | http://localhost:15672 | bugbuster / bugbuster |
| MailHog | http://localhost:8025 | — |
| API Gateway | http://localhost:8888 | — |

## TUI Controls

```
Dashboard:
  1-7     Quick-select action
  j/k     Navigate action list
  enter   Select action
  ctrl+c  Quit (tears down containers)

Submit screen:
  j/k     Scroll results
  r       Back to dashboard
  esc     Back

Hints screen:
  enter   Reveal next hint
  esc     Back
```

## Available Scenarios

| Scenario | Level | Time | What You'll Learn |
|---|---|---|---|
| Ghost Latency | Medium | 25 min | Resource saturation, USE method, thread pool exhaustion |

*More scenarios coming soon.*

## AI Coach (Optional)

If you have [Claude Code](https://claude.ai/download) (`claude`) or [Gemini CLI](https://github.com/google-gemini/gemini-cli) (`gemini`) installed, BugBuster will automatically use it to evaluate your RCA submission and give personalized feedback after you solve a scenario.

## License

MIT
