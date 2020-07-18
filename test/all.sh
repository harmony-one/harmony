#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
bash "$DIR/kill_node.sh"
docker pull harmonyone/localnet-test
docker run -it --expose 9000-9999 -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test