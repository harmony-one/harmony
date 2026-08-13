#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
docker_name="harmony-localnet-test"

case ${1:-} in
run)
    bash "$DIR/kill_node.sh"
    exec bash "$DIR/localnet.sh" --keep mesh
    ;;
attach)
    docker exec -it "$docker_name" /bin/bash
    ;;
*)
    echo "
Mesh API tests

Param:     Help:
run        Run the Mesh API tests
attach     Attach to the Mesh API testing container
"
    exit 0
    ;;
esac
