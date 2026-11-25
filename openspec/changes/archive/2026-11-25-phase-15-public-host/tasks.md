# Tasks

## 1. Decision

- [x] 1.1 Decision record 0023.

## 2. Host definition

- [x] 2.1 `deploy/vps/compose.yaml`, `Caddyfile`, `vps.env.example`, `.gitignore` entry (`compose_test.go`: rendered for the host, missing password hash, Caddy configuration).
- [x] 2.2 `setup-server.sh`, `init-env.sh`, `deploy.sh`.

## 3. Deployment

- [x] 3.1 Prepare the host, write its secrets, deploy, and train version 1.
- [x] 3.2 Check the running host (`compose_test.go`: running host); README section.

## Results

Host: OVHcloud VPS-1 (2 vCores, 4 GB RAM, 40 GB disk), Ubuntu 26.04, Docker 29.1.3 and Compose 2.40.3 from Ubuntu's archive, 2 GiB swap, at `167-114-182-175.sslip.io`.

- `deploy.sh` from empty volumes: images pulled and every service running, with Caddy holding a Let's Encrypt certificate for the site within seconds of starting.
- `TestRunningHost` against the host: 401 without credentials on `/` and on `/api/v1/models/eurosat-classifier`, 200 with them, and 308 from HTTP to HTTPS.
- Bootstrap on the host: EuroSAT downloaded and split (18,900 train, 4,050 val, 4,050 holdout); 3 epochs in 1,014 s (holdout accuracy 0.9069, macro F1 0.902); version 1 registered in shadow 18 min 50 s after the run started. The worker began shadowing it on the next reload. During training the host used about 2.4 GiB of RAM and under 50 MiB of swap.
- Version 1 is not promoted: approving and promoting it is a person's step in the dashboard.
