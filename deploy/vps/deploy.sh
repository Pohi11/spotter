#!/bin/sh
# Ships the deployment files to the public demo host and starts the platform there
# (decision record 0023). Run from the repository root:
#   sh deploy/vps/deploy.sh ubuntu@<host>
# The host needs setup-server.sh and init-env.sh run once first. Images come from GHCR
# (IMAGE_TAG in vps.env, default main); nothing is built on the host.
set -eu

host=${1:?usage: deploy.sh <user@host>}
cd "$(dirname "$0")/../.."

files="deploy/demo/compose.yaml
deploy/helm/spotter/files/seaweedfs-entrypoint.sh
deploy/vps/compose.yaml
deploy/vps/Caddyfile
deploy/vps/vps.env.example
deploy/vps/setup-server.sh
deploy/vps/init-env.sh"

# shellcheck disable=SC2086
tar -cf - $files | ssh "$host" 'mkdir -p ~/spotter && tar -xf - -C ~/spotter'

# The retrainer subscribes to the monitor's stream, so it starts after the platform is up.
ssh "$host" 'set -e
  cd ~/spotter/deploy/vps
  test -e vps.env || { echo "no vps.env on the host; run init-env.sh there first" >&2; exit 1; }
  dc() { docker compose -f ../demo/compose.yaml -f compose.yaml --env-file vps.env "$@"; }
  dc --profile retrain pull -q
  dc up -d --wait --remove-orphans
  dc --profile retrain up -d retrainer
  dc --profile retrain ps --format "table {{.Service}}\t{{.Status}}"'
