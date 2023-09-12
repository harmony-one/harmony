#!/usr/bin/env bash
set -e

echo $TRAVIS_PULL_REQUEST_BRANCH
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo $DIR
echo $GOPATH
cd $GOPATH/src/github.com/servprotocolorg/harmony-test
git fetch
git checkout $TRAVIS_PULL_REQUEST_BRANCH || true
git pull
git branch --show-current
cd localnet
docker build -t harmonyone/localnet-test .
docker run -v "$DIR/../:/go/src/github.com/servprotocolorg/harmony" harmonyone/localnet-test -n