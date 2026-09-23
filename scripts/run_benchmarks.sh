#!/usr/bin/env bash
# ==============================================================================
# 🛸 UAV Telemetry Ingestion Benchmark Matrix Orchestrator
# ==============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# --- 0. Parse Arguments & Environment Variables ---
NOTE="${BENCH_NOTE:-}"
TAG="${BENCH_TAG:-}"
SCENARIO_DURATION="${BENCH_DURATION:-5}"

while [[ $# -gt 0 ]]; do
  case $1 in
    --note=*) NOTE="${1#*=}"; shift ;;
    --note) NOTE="$2"; shift 2 ;;
    --tag=*) TAG="${1#*=}"; shift ;;
    --tag) TAG="$2"; shift 2 ;;
    --duration=*) SCENARIO_DURATION="${1#*=}"; shift ;;
    --duration) SCENARIO_DURATION="$2"; shift 2 ;;
    *) shift ;;
  esac
done

BENCH_DB="uav_telemetry_bench"
TARGET_ADDR="127.0.0.1:9876"
SUITE_START_TIME=$(date +%s)
TIMESTAMP="$(date -u '+%Y%m%d_%H%M%S')"

RUN_NAME="${TIMESTAMP}"
if [[ -n "$TAG" ]]; then
    CLEAN_TAG=$(echo "$TAG" | tr -cs 'a-zA-Z0-9_-' '_' | sed 's/_$//;s/^_//')
    RUN_NAME="${TIMESTAMP}_${CLEAN_TAG}"
fi

RUNS_DIR="benchmarks/runs"
CURRENT_RUN_DIR="$RUNS_DIR/$RUN_NAME"
mkdir -p "$CURRENT_RUN_DIR"

OUTPUT_JSON="$CURRENT_RUN_DIR/benchmark.json"

# --- 1. Defensive Security Guard ---
if [[ "$BENCH_DB" != *"bench"* && "$BENCH_DB" != *"test"* ]]; then
    echo "❌ SECURITY ERROR: Aborting! Database name does not contain 'bench' or 'test': $BENCH_DB"
    exit 1
fi

# --- Git Metadata Extraction ---
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
    GIT_COMMIT="${GIT_COMMIT}+dirty"
fi
HOST_INFO="$(uname -srm) ($(nproc) cores)"

echo "================================================================"
echo " 🛸 UAV Benchmark Suite: High-Throughput Ingestion & TimescaleDB"
echo "================================================================"
echo " Database     : $BENCH_DB (isolated benchmark database)"
echo " Gateway Port : $TARGET_ADDR"
echo " Duration/Run : ${SCENARIO_DURATION}s per scenario"
echo " Git Ref      : $GIT_COMMIT ($GIT_BRANCH)"
if [[ -n "$NOTE" ]]; then
    echo " Run Note     : $NOTE"
fi
echo " Run Directory: $CURRENT_RUN_DIR"
echo " Date         : $(date -u '+%Y-%m-%d %H:%M:%SZ')"
echo "----------------------------------------------------------------"

# --- 2. Ensure TimescaleDB container is running and database exists ---
if ! docker compose -f docker/docker-compose.yml ps --filter "name=uav_timescaledb" --format '{{.Status}}' | grep -qi "up"; then
    echo "🐳 Starting TimescaleDB container..."
    docker compose -f docker/docker-compose.yml up -d timescaledb
    sleep 3
fi

# Ensure database exists
docker compose -f docker/docker-compose.yml exec -T timescaledb psql -U uav_admin -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$BENCH_DB'" | grep -q 1 || \
docker compose -f docker/docker-compose.yml exec -T timescaledb psql -U uav_admin -d postgres -c "CREATE DATABASE $BENCH_DB;"

# --- 3. Build Fresh Binaries ---
make build > /dev/null

# --- 4. Define Symmetrical Paired Scenario Matrix ---
# Format: Name | Drones | FrequencyHz | DurationSeconds | Category
SCENARIOS=(
    # --- Tier 1: 1,000 Hz Target ---
    "10 UAVs @ 100 Hz (Fleet)|10|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 20 Hz (Freq)|50|20|${SCENARIO_DURATION}|frequency"

    # --- Tier 2: 5,000 Hz Target (Reference crossover point) ---
    "50 UAVs @ 100 Hz (Fleet)|50|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 100 Hz (Freq)|50|100|${SCENARIO_DURATION}|frequency"

    # --- Tier 3: 10,000 Hz Target ---
    "100 UAVs @ 100 Hz (Fleet)|100|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 200 Hz (Freq)|50|200|${SCENARIO_DURATION}|frequency"

    # --- Tier 4: 25,000 Hz Target ---
    "250 UAVs @ 100 Hz (Fleet)|250|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 500 Hz (Freq)|50|500|${SCENARIO_DURATION}|frequency"

    # --- Tier 5: 50,000 Hz Target ---
    "500 UAVs @ 100 Hz (Fleet)|500|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 1000 Hz (Freq)|50|1000|${SCENARIO_DURATION}|frequency"

    # --- Tier 6: 100,000 Hz Target ---
    "1000 UAVs @ 100 Hz (Fleet)|1000|100|${SCENARIO_DURATION}|fleet"
    "50 UAVs @ 2000 Hz (Freq)|50|2000|${SCENARIO_DURATION}|frequency"
)

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

JSON_ENTRIES=()

for SCENARIO in "${SCENARIOS[@]}"; do
    IFS="|" read -r NAME DRONES FREQ DURATION CATEGORY <<< "$SCENARIO"
    TARGET_HZ=$(( DRONES * FREQ ))

    echo ""
    echo "▶️  Running Scenario: [$NAME] (Target: ${TARGET_HZ} Hz for ${DURATION}s)..."

    # Truncate benchmark database safely
    docker compose -f docker/docker-compose.yml exec -T timescaledb psql -U uav_admin -d "$BENCH_DB" -c "TRUNCATE telemetry, device_events;" > /dev/null

    GW_LOG="$TMP_DIR/gw.log"
    SIM_LOG="$TMP_DIR/sim.log"

    # Start Gateway in background
    ./bin/uav gateway --db-name="$BENCH_DB" --listen-addr="$TARGET_ADDR" > "$GW_LOG" 2>&1 &
    GW_PID=$!
    sleep 1.2

    # Start Swarm Generator in background
    ./bin/uav sim --target-addr="$TARGET_ADDR" --drones="$DRONES" --frequency="$FREQ" > "$SIM_LOG" 2>&1 &
    SIM_PID=$!

    # Run for requested duration
    sleep "$DURATION"

    # Stop Swarm cleanly
    kill -TERM "$SIM_PID" 2>/dev/null || true
    wait "$SIM_PID" 2>/dev/null || true
    sleep 1.0

    # Stop Gateway cleanly (flushes remaining DB queue)
    kill -TERM "$GW_PID" 2>/dev/null || true
    wait "$GW_PID" 2>/dev/null || true

    # Parse Simulator output
    SENT_PACKETS=$(grep "Paquetes emitidos" "$SIM_LOG" | tail -n 1 | awk '{print $4}' || echo "0")
    SENT_BYTES=$(grep "Bytes transmitidos" "$SIM_LOG" | tail -n 1 | awk '{print $4}' || echo "0")
    SENT_ERRORS=$(grep "Errores de red UDP" "$SIM_LOG" | tail -n 1 | awk '{print $6}' || echo "0")
    ACTUAL_TX_HZ=$(grep "Tasa media global" "$SIM_LOG" | tail -n 1 | awk '{print $5}' || echo "0")
    TX_BANDWIDTH=$(grep "Ancho de banda" "$SIM_LOG" | tail -n 1 | awk '{print $5}' || echo "0")

    # Parse Gateway output
    RECV_PACKETS=$(grep "Paquetes procesados" "$GW_LOG" | tail -n 1 | awk '{print $4}' || echo "0")
    RECV_BYTES=$(grep "Bytes ingeridos" "$GW_LOG" | tail -n 1 | awk '{print $4}' || echo "0")
    UDP_LOSS=$(grep "Pérdidas en red UDP" "$GW_LOG" | tail -n 1 | awk '{print $6}' || echo "0")
    TRANSIT_LATENCY=$(grep "Latencia tránsito" "$GW_LOG" | tail -n 1 | awk '{print $4}' || echo "0")

    # Parse Database Persistence output
    PERSIST_LINE=$(grep "Persistidos en BD" "$GW_LOG" | tail -n 1 || echo "")
    DB_PERSISTED=$(echo "$PERSIST_LINE" | awk '{print $5}' || echo "0")
    DB_FLUSHES=$(echo "$PERSIST_LINE" | sed -n 's/.*(\([0-9]*\) flushes.*/\1/p' || echo "0")
    DB_ERRORS=$(echo "$PERSIST_LINE" | sed -n 's/.*, \([0-9]*\) errores.*/\1/p' || echo "0")
    DB_DROPS=$(echo "$PERSIST_LINE" | sed -n 's/.*, \([0-9]*\) drops.*/\1/p' || echo "0")

    # Sanitization
    SENT_PACKETS=${SENT_PACKETS:-0}
    SENT_BYTES=${SENT_BYTES:-0}
    SENT_ERRORS=${SENT_ERRORS:-0}
    RECV_PACKETS=${RECV_PACKETS:-0}
    RECV_BYTES=${RECV_BYTES:-0}
    DB_PERSISTED=${DB_PERSISTED:-0}
    DB_FLUSHES=${DB_FLUSHES:-0}
    DB_ERRORS=${DB_ERRORS:-0}
    DB_DROPS=${DB_DROPS:-0}
    UDP_LOSS=${UDP_LOSS:-0}
    ACTUAL_TX_HZ=${ACTUAL_TX_HZ:-0}
    TRANSIT_LATENCY=${TRANSIT_LATENCY:-0}
    TX_BANDWIDTH=${TX_BANDWIDTH:-0}

    # Calculate Rates using awk to ensure standards-compliant JSON float formatting (leading zeros)
    ACTUAL_RX_HZ=$(awk -v r="$RECV_PACKETS" -v d="$DURATION" 'BEGIN {if (d > 0) printf "%.2f", r / d; else printf "0.00"}')
    DB_PERSIST_RATE=$(awk -v p="$DB_PERSISTED" -v d="$DURATION" 'BEGIN {if (d > 0) printf "%.2f", p / d; else printf "0.00"}')
    UDP_LOSS_PCT=$(awk -v l="$UDP_LOSS" -v s="$SENT_PACKETS" 'BEGIN {if (s > 0) printf "%.3f", (l * 100.0) / s; else printf "0.000"}')
    TOTAL_ENQUEUED=$(( DB_PERSISTED + DB_DROPS ))
    DB_DROP_PCT=$(awk -v d="$DB_DROPS" -v t="$TOTAL_ENQUEUED" 'BEGIN {if (t > 0) printf "%.2f", (d * 100.0) / t; else printf "0.00"}')
    TX_BW_MBS=$(awk -v b="$TX_BANDWIDTH" 'BEGIN {printf "%.2f", b / 1024.0}')

    echo "   ✅ Tx: ${ACTUAL_TX_HZ} Hz | Rx: ${ACTUAL_RX_HZ} Hz | Latency: ${TRANSIT_LATENCY} ms | DB Persist: ${DB_PERSIST_RATE} rows/s (Drops: ${DB_DROP_PCT}%)"

    JSON_ENTRY=$(cat <<EOF
    {
      "name": "$NAME",
      "category": "$CATEGORY",
      "drones": $DRONES,
      "frequency_hz": $FREQ,
      "target_hz": $TARGET_HZ,
      "duration_sec": $DURATION,
      "sent_packets": ${SENT_PACKETS},
      "sent_bytes": ${SENT_BYTES},
      "sent_errors": ${SENT_ERRORS},
      "actual_tx_hz": ${ACTUAL_TX_HZ},
      "tx_bandwidth_mbs": ${TX_BW_MBS},
      "recv_packets": ${RECV_PACKETS},
      "recv_bytes": ${RECV_BYTES},
      "recv_errors": 0,
      "actual_rx_hz": ${ACTUAL_RX_HZ},
      "transit_latency_ms": ${TRANSIT_LATENCY},
      "udp_loss_count": ${UDP_LOSS},
      "udp_loss_percent": ${UDP_LOSS_PCT},
      "db_persisted": ${DB_PERSISTED},
      "db_flushes": ${DB_FLUSHES},
      "db_errors": ${DB_ERRORS},
      "db_drops": ${DB_DROPS},
      "db_persist_rate": ${DB_PERSIST_RATE},
      "db_drop_percent": ${DB_DROP_PCT}
    }
EOF
)
    JSON_ENTRIES+=("$JSON_ENTRY")
done

# --- 5. Consolidate JSON dataset ---
echo ""
echo "📝 Consolidating benchmark dataset..."

SUITE_END_TIME=$(date +%s)
TOTAL_SUITE_DURATION=$(( SUITE_END_TIME - SUITE_START_TIME ))

IFS=","
JOINED_ENTRIES="${JSON_ENTRIES[*]}"

# Escape note for JSON
SAFE_NOTE=$(echo "$NOTE" | sed 's/"/\\"/g')
SAFE_TAG=$(echo "$TAG" | sed 's/"/\\"/g')

cat <<EOF > "$OUTPUT_JSON"
{
  "timestamp": "$(date -u '+%Y-%m-%d %H:%M:%SZ')",
  "git_commit": "$GIT_COMMIT",
  "git_branch": "$GIT_BRANCH",
  "note": "$SAFE_NOTE",
  "tag": "$SAFE_TAG",
  "host_info": "$HOST_INFO",
  "total_duration_sec": ${TOTAL_SUITE_DURATION}.0,
  "scenarios": [
$JOINED_ENTRIES
  ]
}
EOF

echo "💾 Saved dataset to: $OUTPUT_JSON"

# --- 6. Generate Markdown Report and SVG Charts ---
echo "📊 Generating report and SVG visualizations in $CURRENT_RUN_DIR..."
go run ./scripts/report/main.go "$OUTPUT_JSON" "$CURRENT_RUN_DIR"

echo ""
echo "================================================================"
echo " 🎉 Benchmark Suite Finished Successfully!"
echo " Total Execution Time: ${TOTAL_SUITE_DURATION} seconds ($(( TOTAL_SUITE_DURATION / 60 ))m $(( TOTAL_SUITE_DURATION % 60 ))s)"
echo " Run Directory       : $CURRENT_RUN_DIR"
echo " Results available in:"
echo "   - $CURRENT_RUN_DIR/REPORT.md"
echo "   - $CURRENT_RUN_DIR/throughput_scaling.svg"
echo "   - $CURRENT_RUN_DIR/db_bottleneck.svg"
echo "================================================================"
