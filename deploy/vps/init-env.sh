#!/bin/sh
# Writes vps.env on the host with random secrets (decision record 0023). Run once, on the
# host, from deploy/vps/:  sh init-env.sh <site-address>
# An existing vps.env is never overwritten. The site password is generated here and stored
# only on the host, in ~/spotter-site-password.txt (mode 600); vps.env holds its bcrypt hash.
set -eu

site=${1:?usage: init-env.sh <site-address>}
cd "$(dirname "$0")"
if [ -e vps.env ]; then
  echo "vps.env exists; leaving it as it is"
  exit 0
fi

rand() { openssl rand -hex "$1"; }
password=$(openssl rand -base64 18 | tr '+/' '-_')
hash=$(docker run --rm caddy:2.11.9-alpine caddy hash-password --plaintext "$password")

umask 077
cat >vps.env <<EOF
POSTGRES_PASSWORD=$(rand 24)
S3_ACCESS_KEY=$(rand 10)
S3_SECRET_KEY=$(rand 24)
SITE_ADDRESS=$site
BASIC_AUTH_USER=spotter
BASIC_AUTH_HASH='$hash'
EOF
printf 'user: spotter\npassword: %s\n' "$password" >"$HOME/spotter-site-password.txt"
echo "wrote vps.env; the site login is in ~/spotter-site-password.txt"
