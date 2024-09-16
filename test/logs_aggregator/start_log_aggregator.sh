#!/usr/bin/env bash

set -eou pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo "working in ${DIR}"
cd $DIR && pwd
logs="$(ls -1 '../../tmp_log/')"
path="$(readlink -f '../../tmp_log/')"
echo "Current localnet logs are placed into '${path}/${logs}'"
echo "CURRENT_SESSION_LOGS='${path}/${logs}'" > .env
echo "CURRENT_SESSION_LOGS='${path}/${logs}'"
echo "starting docker compose lor log aggregation"
docker-compose up --detach
sleep 5
echo "Opening Grafana"
python3 -m webbrowser "http://localhost:3000/explore"
