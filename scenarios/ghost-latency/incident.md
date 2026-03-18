# INCIDENT: Slow API Response Times

> *Based on a real incident — New Year's Eve 2016, a food delivery platform
> for a QSR chain. Festival traffic spiked 10x. No crashes, no errors, just
> customers staring at spinning loaders. It took the team 3 hours to find it.
> Let's see if you can do it faster.*

**Severity:** P2
**Time detected:** 3 minutes ago
**Affected:** All API endpoints (primarily payment-related)
**On-call:** You

---

## Alert

> **[ALERT] Response Time Degradation**
> Source: Synthetic monitoring
> Trigger: p95 latency > 3000ms for 2 consecutive checks
> Affected service: api-gateway (all downstream routes)

---

## Customer Reports

Three support tickets opened in the last 5 minutes:

- *"Checkout page just spins forever. I've been waiting 30 seconds and nothing happens."*
- *"The product page loaded fine but when I click 'Place Order' it hangs."*
- *"API calls from our mobile app are timing out. Started about 10 minutes ago."*

---

## What We Know

- **No recent code changes** were deployed to the affected services
- **Infrastructure config was updated** as part of a routine capacity tuning exercise
- **No errors** are appearing in application logs — services report healthy
- **Health checks** are passing on all services
- **CPU and memory** look normal across all containers
- **Database query times** are normal

---

## Your Mission

Identify the root cause of the latency spike and restore normal response times.

---

## Your Toolkit

### Observability UIs

| Tool | URL | Credentials |
|------|-----|-------------|
| Grafana | http://localhost:3000 | admin / bugbuster |
| Jaeger | http://localhost:16686 | — |
| Prometheus | http://localhost:9091 | — |
| RabbitMQ | http://localhost:15672 | bugbuster / bugbuster |
| MailHog | http://localhost:8025 | — |

### Test the APIs yourself

```bash
# Browse products (should be fast)
curl -w "\n  Time: %{time_total}s\n" http://localhost:8888/api/catalog/products

# Place an order (is this slow?)
curl -w "\n  Time: %{time_total}s\n" -X POST http://localhost:8888/api/orders \
  -H 'Content-Type: application/json' \
  -d '{"product_id": 1, "quantity": 1}'

# Check an order
curl -w "\n  Time: %{time_total}s\n" http://localhost:8888/api/orders/1

# Hit the payment service directly (bypass gateway)
curl -w "\n  Time: %{time_total}s\n" http://localhost:3003/actuator/health

# Simulate concurrent load (10 parallel orders)
for i in $(seq 1 10); do
  curl -s -o /dev/null -w "req$i: %{time_total}s\n" \
    -X POST http://localhost:8888/api/orders \
    -H 'Content-Type: application/json' \
    -d "{\"product_id\": $((RANDOM % 10 + 1)), \"quantity\": 1}" &
done
wait
```

### Read service logs

```bash
# All services
docker compose logs --tail 50

# Specific service
docker compose logs --tail 50 payment-service
docker compose logs --tail 50 order-service
docker compose logs --tail 50 catalog-service

# Follow logs live
docker compose logs -f payment-service
```

### Query Prometheus (useful starting points)

```bash
# Service health - are all targets up?
curl -s 'http://localhost:9091/api/v1/query?query=up' | python3 -m json.tool

# HTTP request rate per service
curl -s 'http://localhost:9091/api/v1/query?query=rate(http_server_requests_seconds_count[1m])' | python3 -m json.tool

# HTTP latency (avg) per endpoint
curl -s 'http://localhost:9091/api/v1/query?query=rate(http_server_requests_seconds_sum[1m])/rate(http_server_requests_seconds_count[1m])' | python3 -m json.tool

# JVM thread count
curl -s 'http://localhost:9091/api/v1/query?query=jvm_threads_live_threads' | python3 -m json.tool

# Database connection pool
curl -s 'http://localhost:9091/api/v1/query?query=hikaricp_connections_active' | python3 -m json.tool

# Explore all available metric names
curl -s 'http://localhost:9091/api/v1/label/__name__/values' | python3 -m json.tool | head -50
```

### Inspect containers

```bash
# Check environment variables of a service
docker exec bugbuster-payment-service-1 env | sort

# Check resource usage
docker stats --no-stream

# Check container processes
docker exec bugbuster-payment-service-1 ps aux
```

### Check Jaeger traces

Open http://localhost:16686, select a service from the dropdown, and click "Find Traces".
Look for:
- Which service has the longest spans?
- Are there gaps between spans (time spent waiting)?
- Compare fast traces vs slow traces — what's different?

---

## When You're Ready

When you think you've found and fixed the issue:

```bash
# Validate your fix
bash scenarios/ghost-latency/validate.sh

# Or submit via CLI
./bugbuster submit
```

Good luck.
