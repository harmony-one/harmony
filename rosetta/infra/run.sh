#!/usr/bin/env bash
set -e

# TODO: implement running a node in offline mode b4 continuing

./harmony -n testnet --http --http.ip 0.0.0.0 --http.rosetta --node_type=explorer --shard_id=0 --run.archive --ws.ip 0.0.0.0 --datadir ./data --log.dir ./data/logs