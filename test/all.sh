#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
bash "$DIR/kill_node.sh"

bash "$DIR/go.sh"
bash "$DIR/localnet.sh" rpc mesh pyhmy
