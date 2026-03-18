# BugBuster - Microservices Debugging Playground

> "You wouldn't train a pilot in a simulator that never crashes."

BugBuster is an open-source, production-like microservices environment where engineers practice debugging real distributed systems problems — manually, with real observability tools, on their own laptop.

## Why?

Junior engineers learn debugging by struggling through incidents. Reading postmortems isn't enough — you need muscle memory. BugBuster creates safe, repeatable incidents with pre-built observability so engineers can practice the **USE method** (Utilization, Saturation, Errors) and systematic elimination.

## Architecture

```
                    ┌──────────────────────────────┐
                    │      BUGBUSTER CLI (Go)       │
                    │  start / hint / submit / score│
                    └──────────┬───────────────────┘
                               │ orchestrates
                    ┌──────────▼───────────────────┐
                    │      DOCKER COMPOSE           │
                    │                               │
                    │  ┌─────────────────────────┐  │
                    │  │     API GATEWAY (nginx)  │  │
                    │  └────┬──────┬────────┬────┘  │
                    │       │      │        │       │
                    │  ┌────▼──┐ ┌─▼────┐ ┌─▼────┐ │
                    │  │ ORDER │ │CATALOG│ │ PAY  │ │
                    │  │ (Node)│ │ (Py)  │ │(Java)│ │
                    │  └──┬────┘ └──┬────┘ └──┬───┘ │
                    │     │   ┌─────┤         │     │
                    │  ┌──▼───▼──┐ ┌▼─────┐ ┌─▼──┐ │
                    │  │RabbitMQ │ │ Redis │ │Mock │ │
                    │  └────┬────┘ └──────┘ │ API │ │
                    │  ┌────▼────┐          └────┘  │
                    │  │NOTIFIER │                   │
                    │  │  (Go)   │                   │
                    │  └─────────┘                   │
                    │                               │
                    │  ┌──────┐ ┌──────┐ ┌──────┐  │
                    │  │ DB 1 │ │ DB 2 │ │ DB 3 │  │
                    │  └──────┘ └──────┘ └──────┘  │
                    │                               │
                    │  ── OBSERVABILITY ──────────  │
                    │  Jaeger │ Grafana+Loki │Prom  │
                    └──────────────────────────────┘
```

## Services

| Service | Language | Port | Role |
|---|---|---|---|
| API Gateway | nginx | 8080 | Routing, request ID propagation |
| Order Service | Node.js/Express | 3001 | Create/list orders, publishes to RabbitMQ |
| Catalog Service | Python/FastAPI | 3002 | Product listing, Redis cache layer |
| Payment Service | Java/Spring Boot | 3003 | Process payments, external API calls |
| Notifier Service | Go | - | Queue consumer, sends email via SMTP |

## Observability

| Tool | Port | Purpose |
|---|---|---|
| Grafana | 3000 | Dashboards (admin/bugbuster) |
| Jaeger | 16686 | Distributed tracing |
| Prometheus | 9091 | Metrics |
| Loki | 3100 | Log aggregation |
| RabbitMQ UI | 15672 | Queue management (bugbuster/bugbuster) |
| MailHog | 8025 | Email viewer |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for building the CLI)

### Install CLI

```bash
cd bugbuster
go build -o bugbuster ./cmd/bugbuster
```

### Start a Scenario

```bash
# List available scenarios
./bugbuster list

# Start a scenario (boots all services + injects the bug)
./bugbuster start ghost-latency

# You'll see an incident alert — now debug it!
# Use Grafana (localhost:3000), Jaeger (localhost:16686), logs, etc.

# Need a hint? (-50 points each)
./bugbuster hint

# Check status
./bugbuster status

# Found the root cause? Submit your answer
./bugbuster submit

# Done — tear down
./bugbuster stop

# Check your score
./bugbuster leaderboard
```

### Run Healthy (no bug)

```bash
# Just boot the healthy system to explore
docker compose up -d --build
docker compose -f docker-compose.observability.yml up -d

# Enable load generator for live dashboards
docker compose --profile loadgen up -d
```

## Scenarios

| Scenario | Level | Time | Category |
|---|---|---|---|
| Ghost Latency | 2 | 25 min | Resource Saturation |

*More scenarios coming soon.*

## Scoring

- Start with **1000 points**
- Each hint costs **50 points**
- Time penalty: **10 points/minute** over estimated time
- Correct RCA category: bonus points
- Validation must pass (automated load test)

## How Scenarios Work

Each scenario is a directory under `scenarios/` containing:

```
scenarios/<name>/
  scenario.yaml         # Bug definition + injection config
  incident.md           # What you see (the alert)
  hints.yaml            # Progressive hints (cost points)
  solution.md           # Expected RCA (hidden until done)
  compose.override.yaml # Docker Compose overrides (injects the bug)
  validate.sh           # Automated verification
```

Bugs are injected via environment variables and Docker Compose overrides — no source code changes needed.

## Contributing Scenarios

1. Create a new directory under `scenarios/`
2. Define the bug injection in `compose.override.yaml` (env vars, resource limits, etc.)
3. Write a realistic incident report in `incident.md`
4. Add progressive hints in `hints.yaml`
5. Document the full solution in `solution.md`
6. Write validation in `validate.sh`

See `scenarios/ghost-latency/` as a reference.

## Debugging Methodology: The USE Method

For every resource (CPU, memory, thread pools, connection pools, queues, file descriptors):

| Signal | Question |
|---|---|
| **U**tilization | What percentage of the resource is in use? |
| **S**aturation | Is there a queue/backlog waiting for this resource? |
| **E**rrors | Are there errors related to this resource? |

This is the systematic approach BugBuster teaches. Don't guess — measure.

## License

MIT
