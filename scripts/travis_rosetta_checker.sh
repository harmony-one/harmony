#!/usr/bin/env bash
set -e

echo $TRAVIS_PULL_REQUEST_BRANCH
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo $DIR
echo $GOPATH
pwd
docker build --progress=plain -t harmonyone/localnet-test -f Tests.Dockerfile .
docker run -v "$DIR/tmp_log:/go/src/github.com/harmony-one/harmony/tmp_log" harmonyone/localnet-test -r