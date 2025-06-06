#!/usr/bin/env bash

set -eou pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo "working in ${DIR}"
cd $DIR && pwd
path="$(readlink -f '../../tmp_log/')"
test -d "${path}" || (echo "logs folder do not exist - ${path},"\
    "please create a localnet first, exiting" && exit 1)
log_path="$(find ${path} -type d | sort | tail -n 1)"
log_path=$(echo "${log_path}" | sed "s#.*/tmp_log/#/var/log/#")
loki_path="${DIR}/loki"
mkdir -p "${loki_path}"
echo "LOG_FOLDER='${path}'" > .env
echo "LOKI_FOLDER='${loki_path}'" >> .env
echo "starting docker compose for log aggregation"
docker compose up --detach
sleep 5
echo "Whole list of log folders"
find ${path} -type d | sort | grep 'log-'
echo "Opening Grafana"
python3 -m webbrowser "http://localhost:3000/explore"
echo "Please, use {filename=~\"${log_path}/.*\"} to get the latest run localnet logs"
