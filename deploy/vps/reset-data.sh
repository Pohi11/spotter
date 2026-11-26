#!/bin/sh
# Wipes the public demo host's platform data and trains a fresh version 1 (decision record
# 0023). Run from the repository root:
#   sh deploy/vps/reset-data.sh ubuntu@<host>
#
# Deletes, permanently: the registry database (every version, approval, and stage change),
# the NATS streams, and the object store (every model file). Keeps the downloaded EuroSAT
# dataset (traindata) and Caddy's certificate (caddydata, caddyconfig), so nothing is
# downloaded or re-issued. The bootstrap runs detached on the host with BOOTSTRAP_EPOCHS from
# vps.env and logs to ~/bootstrap.log; the retrainer stays stopped until the next deploy.sh,
# so the two training runs never share the host's CPU.
set -eu

host=${1:?usage: reset-data.sh <user@host>}

ssh "$host" 'set -e
  cd ~/spotter/deploy/vps
  dc() { docker compose -f ../demo/compose.yaml -f compose.yaml --env-file vps.env "$@"; }
  dc --profile retrain --profile bootstrap down --remove-orphans
  docker volume rm spotter-demo_pgdata spotter-demo_natsdata spotter-demo_seaweeddata
  dc up -d --wait
  echo "bootstrap epochs: $(sed -n "s/^BOOTSTRAP_EPOCHS=//p" vps.env)"
  nohup sh -c "docker compose -f ../demo/compose.yaml -f compose.yaml --env-file vps.env --profile bootstrap run --rm bootstrap" \
    >"$HOME/bootstrap.log" 2>&1 </dev/null &
  echo "bootstrap started; follow it with: tail -f ~/bootstrap.log"'
