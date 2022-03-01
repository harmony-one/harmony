#!/usr/bin/env bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
DATA="$DIR/data"
LOGS="$DATA/logs"
BASE_ARGS=(--http.ip "0.0.0.0" --ws.ip "0.0.0.0" --http.rosetta --node_type "explorer" --datadir "$DATA" --log.dir "$LOGS")

if [ -n "$RCLONE_DB_0_URL" ]; then
  rclone -P -L sync $RCLONE_DB_0_URL $DATA/harmony_db_0 --multi-thread-streams 4 --transfers=8
fi

mkdir -p "$LOGS"
echo -e NODE ARGS: \" "$@"  "${BASE_ARGS[@]}" \"
echo "NODE VERSION: $(./harmony --version)"
"$DIR/harmony" "$@" "${BASE_ARGS[@]}"
