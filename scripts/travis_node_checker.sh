#!/usr/bin/env bash
set -e
OS=$(uname -s)
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo "$(pwd)"
docker pull harmonyone/localnet-test
docker run -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -n