# Solution: Ghost Latency

## Root Cause

The `payment-service` has two misconfigurations working together:

1. **`SERVER_TOMCAT_THREADS_MAX=8`** — Thread pool is undersized for 20+ concurrent users
2. **`SERVER_TOMCAT_ACCEPT_COUNT=1000`** — TCP accept queue is massive

When all 8 threads are busy (each blocking on the external payment API call ~200ms), new requests silently pile up in the 1000-deep accept queue. The server never rejects anything — it just gets slower and slower.

```
              Incoming Requests (20+ concurrent)
                    |
                    v
         +-------------------+
         |  TCP Accept Queue  |  accept-count = 1000
         |  [996 slots free]  |  <-- requests wait here silently
         +-------------------+
                    |
           (wait for a thread)     <-- THIS is where latency comes from
                    |
                    v
         +-------------------+
         | Tomcat Thread Pool |  max = 8
         | [T1][T2]...[T8]  |  <-- all busy calling external API
         +-------------------+
                    |
                    v
            Handle Request (~200ms actual work)
```

This is particularly insidious because:
- No errors appear in logs (requests aren't rejected, just delayed)
- Health checks pass (they only need 1 thread)
- CPU and memory look fine (threads are mostly I/O-waiting, not computing)
- The server "looks healthy" to every monitoring check that doesn't measure saturation

## How to Find It

### Step 1: Apply the USE Method

The USE method (Brendan Gregg) says: for every resource, check **Utilization**, **Saturation**, and **Errors**.

| Resource           | Utilization        | Saturation           | Errors |
|--------------------|--------------------|-----------------------|--------|
| CPU                | ~15% (normal)      | None                 | None   |
| Memory             | ~40% (normal)      | None                 | None   |
| Disk               | Normal             | None                 | None   |
| **Tomcat Threads** | **100% (8/8)**     | **Queue depth growing** | None |
| DB Connections     | Normal             | None                 | None   |

The key insight: **Saturation without Errors** = silent queueing.

### Step 2: Check Tomcat Metrics in Grafana

Open the "BugBuster Overview" dashboard and look at the **Tomcat Thread Pool** panel:

- `tomcat_threads_config_max_threads` = **8**
- `tomcat_threads_busy_threads` = **8** (fully saturated)
- The busy line is flat against the max line — 100% utilization

### Step 3: Confirm with Prometheus

```promql
# Max configured threads
tomcat_threads_config_max_threads{job="payment-service"}
# Result: 8

# Currently busy threads
tomcat_threads_busy_threads{job="payment-service"}
# Result: 8 (100% utilization = saturated)

# Compare with latency — avg response time climbing
rate(http_server_requests_seconds_sum{job="payment-service"}[1m])
  / rate(http_server_requests_seconds_count{job="payment-service"}[1m])
```

### Step 4: Observe in Jaeger

Traces for payment-service requests show unusually long gaps at the start of the span — this is the time spent waiting in the TCP queue before a thread picks up the request. The actual processing time (~200ms) is normal.

## The Fix

**Two changes are needed** — this is the key learning:

### Fix 1: Right-size the thread pool

Increase `SERVER_TOMCAT_THREADS_MAX` to handle expected concurrency:

```bash
# In the compose override or via environment
SERVER_TOMCAT_THREADS_MAX=200
```

### Fix 2: Keep the accept queue minimal (the real lesson)

Lower `SERVER_TOMCAT_ACCEPT_COUNT` to a minimal value so the server **rejects fast** instead of **queuing silently**:

```bash
# Keep queue minimal — fail fast, don't silently absorb
SERVER_TOMCAT_ACCEPT_COUNT=5
```

**The queue should be as small as possible.** A large accept queue is a lie — it tells the outside world "I'm handling this" when really it's just hiding the problem. Keep it minimal (5-10) so overload becomes visible immediately.

Why this matters: with a minimal accept queue, when the server is overwhelmed it returns **503 immediately**. This enables:
- Load balancers to route to healthy instances
- Autoscalers to trigger based on rejection rate
- Clients to retry with backoff
- Monitoring to fire alerts on error rate spikes
- Engineers to see the problem in dashboards within seconds, not hours

**Fail fast is always better than fail slow. A server that rejects quickly is healthier than one that silently queues — because rejection is visible, measurable, and actionable.**

### Apply the fix

```bash
# Edit scenarios/ghost-latency/compose.override.yaml:
#   SERVER_TOMCAT_THREADS_MAX: "200"
#   SERVER_TOMCAT_ACCEPT_COUNT: "20"

# Then restart payment-service
docker compose restart payment-service
```

## Validate

```bash
bash scenarios/ghost-latency/validate.sh
```

This sends 20 concurrent requests and checks that p95 latency is under 1000ms.

## Key Takeaways

1. **Silent queueing is the hardest failure mode to debug.** No errors, no crashes — just latency. Always check resource saturation, not just errors.

2. **The USE method is your best friend.** Systematically walk through every resource (CPU, memory, threads, connections, disk) and check Utilization, Saturation, Errors.

3. **Fail fast > Fail slow.** A large accept queue hides problems. A server that rejects quickly (503) is healthier than one that silently queues — because rejection is visible, measurable, and actionable.

4. **Thread pools + queue depth = capacity planning.** The combination of these two settings determines how your server behaves under overload. Threads handle concurrency, queue depth determines whether overflow is visible or hidden.

5. **Config changes are deploys too.** "No recent code changes" doesn't mean "nothing changed." Someone tuning server parameters during a "performance optimization" can introduce exactly this kind of bug.

---

## Fun Fact: Thread Pool + HPA = Best Friends

Here's a pro tip most engineers learn the hard way: **set your thread pool max so that when all threads are busy, CPU usage crosses your HPA (Horizontal Pod Autoscaler) threshold.**

```
Example:
  - Each thread consumes ~5% CPU when busy
  - HPA scales at 70% CPU
  - Thread pool max = 14 threads
  - At 14 busy threads: 14 x 5% = 70% CPU --> HPA triggers!

  Result: The pod scales BEFORE the queue builds up.
          Autoscaling and thread pool work in harmony.
```

If your thread pool is too large relative to your HPA threshold, you'll have all threads busy but CPU at only 30% — HPA never fires, and you're back to silent queueing. If it's too small, you waste CPU headroom. The sweet spot: **max threads at full utilization should just barely exceed the HPA CPU target.**

---

## War Story

> *This scenario is based on a real incident. In December 2016, we were running
> a food delivery platform for a QSR chain. New Year's Eve traffic hit — orders
> spiked 10x in 30 minutes. The system didn't crash. It didn't throw errors.
> It just... hung. Dashboards showed everything green. CPU fine. Memory fine.
> No 5xx errors. But customers were staring at spinning loaders and orders
> were timing out on the app side.*
>
> *It took us 3 hours to find it. The Tomcat thread pool was exhausted, and a
> massive accept queue was silently swallowing every request. The server looked
> "healthy" to every health check while hundreds of customers' orders sat in
> a TCP backlog going nowhere.*
>
> *The fix took 2 minutes. The debugging took 3 hours. That's why this scenario
> exists — so you can learn this in 25 minutes instead of 3 hours on New Year's
> Eve with angry customers and cold biryani.*
