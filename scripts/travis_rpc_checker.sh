#!/usr/bin/env bash
set -e

echo $TRAVIS_PULL_REQUEST_BRANCH
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo $DIR
echo $GOPATH
cd tests
echo "$DIR/../:/go/src/github.com/harmony-one/harmony"
docker build -t harmonyone/localnet-test .

docker run -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -n