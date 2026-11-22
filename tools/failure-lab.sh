#!/usr/bin/env bash
# Runs the failure lab (change phase-11-failure-lab): both modes under every fault, one
# dated JSON report per run in benchmarks/. Needs the Compose dependencies, the registry,
# and a gateway started with WORKER_ADDR=127.0.0.1:7061 (the worker is started by
# spotterchaos itself). Usage: tools/failure-lab.sh <directory of holdout .jpg tiles>
set -euo pipefail
cd "$(dirname "$0")/.."
images=${1:?usage: tools/failure-lab.sh <directory of .jpg tiles>}
set -a; . ./.env; set +a
export REGISTRY_ADDR=localhost:9090 MODEL_THREADS=1 SHADOW=off GRPC_PORT=7061
python=.venv/bin/python; [ -x "$python" ] || python=.venv/Scripts/python.exe
date=$(date -u +%Y-%m-%d)
go build -o bin/ ./cmd/spotterchaos
for fault in none worker-kill nats-restart postgres-pause objectstore-pause; do
  for mode in async sync; do
    echo "== $mode / $fault"
    ./bin/spotterchaos -mode "$mode" -fault "$fault" -images "$images" \
      -worker-cmd "$python -m spotter_inference" -worker-log "bin/chaos-worker.log" \
      -out "benchmarks/$date-failure-lab-$mode-$fault.json"
  done
done
