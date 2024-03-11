#!/usr/bin/env bash
set -e

echo $TRAVIS_PULL_REQUEST_BRANCH
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo $DIR
echo $GOPATH
cd $GOPATH/src/github.com/harmony-one/harmony-test
git fetch
git pull
git checkout $TRAVIS_PULL_REQUEST_BRANCH || true
git branch --show-current
cd localnet
docker build -t harmonyone/localnet-test .
docker run -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -r