#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
docker pull harmonyone/localnet-test
docker run -it -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -g
