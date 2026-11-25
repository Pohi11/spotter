#!/bin/sh
# Prepares a fresh Ubuntu host for the public demo (decision record 0023). Idempotent; run on
# the host as a user with sudo:  sh setup-server.sh
#   - Docker Engine and the Compose plugin from Ubuntu's own archive
#   - the current user in the docker group
#   - a 2 GiB swap file when the host has no swap, so a training run cannot take the
#     platform down on a 4 GB machine
set -eu

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  sudo apt-get update -q
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -yq docker.io docker-compose-v2
fi
sudo systemctl enable --now docker

if ! id -nG "$USER" | tr ' ' '\n' | grep -qx docker; then
  sudo usermod -aG docker "$USER"
  echo "added $USER to the docker group; log in again for it to apply"
fi

if [ -z "$(swapon --noheadings)" ]; then
  sudo fallocate -l 2G /swapfile
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile >/dev/null
  sudo swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab >/dev/null
fi

docker --version
docker compose version
swapon --show
