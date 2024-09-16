#!/usr/bin/env bash

set -eou pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo "working in ${DIR}"
cd $DIR && pwd
echo "[INFO] - stopping log aggregation"
docker-compose rm --force --stop --volumes
echo "[INFO] - cleanup .env"
echo > .env
