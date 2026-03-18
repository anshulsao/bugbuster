#!/usr/bin/env bash
#
# Validation script for "The Silent Queue" scenario.
# Sends 20 concurrent requests to the API gateway and checks
# that p95 latency is under 1000ms.
#
set -euo pipefail

TARGET_URL="${TARGET_URL:-http://localhost:8888/api/orders}"
CONCURRENT=20
MAX_P95_MS=1000
TMPDIR_VAL=$(mktemp -d)

echo "=== BugBuster Scenario Validation ==="
echo "Target:      $TARGET_URL"
echo "Concurrent:  $CONCURRENT requests"
echo "Pass if:     p95 < ${MAX_P95_MS}ms"
echo ""

# Send concurrent requests and capture timing
echo "Sending $CONCURRENT concurrent requests..."
for i in $(seq 1 $CONCURRENT); do
  curl -s -o /dev/null \
    -w "%{time_total}" \
    --max-time 30 \
    "$TARGET_URL" \
    > "$TMPDIR_VAL/req_$i.txt" 2>/dev/null &
done

# Wait for all background curl processes
wait

# Collect all response times (in ms)
TIMES=()
FAILURES=0
for i in $(seq 1 $CONCURRENT); do
  FILE="$TMPDIR_VAL/req_$i.txt"
  if [ -s "$FILE" ]; then
    TIME_SEC=$(cat "$FILE")
    TIME_MS=$(echo "$TIME_SEC * 1000" | bc | cut -d'.' -f1)
    TIMES+=("$TIME_MS")
  else
    FAILURES=$((FAILURES + 1))
  fi
done

# Clean up
rm -rf "$TMPDIR_VAL"

if [ ${#TIMES[@]} -eq 0 ]; then
  echo "FAIL: All requests failed. Is the service running?"
  exit 1
fi

# Sort times
IFS=$'\n' SORTED=($(sort -n <<<"${TIMES[*]}")); unset IFS

COUNT=${#SORTED[@]}
P95_INDEX=$(echo "($COUNT * 95 + 99) / 100 - 1" | bc)

# Clamp index
if [ "$P95_INDEX" -ge "$COUNT" ]; then
  P95_INDEX=$((COUNT - 1))
fi
if [ "$P95_INDEX" -lt 0 ]; then
  P95_INDEX=0
fi

P95=${SORTED[$P95_INDEX]}
P50_INDEX=$(( (COUNT * 50 + 99) / 100 - 1 ))
if [ "$P50_INDEX" -lt 0 ]; then P50_INDEX=0; fi
P50=${SORTED[$P50_INDEX]}
MIN=${SORTED[0]}
MAX=${SORTED[$((COUNT - 1))]}

echo ""
echo "--- Results ---"
echo "Successful: $COUNT / $CONCURRENT"
echo "Failed:     $FAILURES / $CONCURRENT"
echo "Min:        ${MIN}ms"
echo "P50:        ${P50}ms"
echo "P95:        ${P95}ms"
echo "Max:        ${MAX}ms"
echo ""

if [ "$P95" -lt "$MAX_P95_MS" ]; then
  echo "PASS: p95 latency ${P95}ms < ${MAX_P95_MS}ms threshold"
  exit 0
else
  echo "FAIL: p95 latency ${P95}ms >= ${MAX_P95_MS}ms threshold"
  echo ""
  echo "The fix hasn't taken effect yet. Keep investigating."
  exit 1
fi
