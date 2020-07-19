#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
bash "$DIR/kill_node.sh"

docker_name="harmony-localnet-test"

case ${1} in
run)
    docker pull harmonyone/localnet-test
    docker rm "$docker_name"
    docker run -it --name "$docker_name" --expose 9000-9999 -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -n -k
    ;;
attach)
    docker exec -it "$docker_name" /bin/bash
    ;;
*)
    echo "
Node API tests

Param:     Help:
run        Run the Node API tests
attach     Attach onto the Node API testing docker image for inspection
"
    exit 0
    ;;
esac

