#!/usr/bin/env bash
set -e

echo $TRAVIS_PULL_REQUEST_BRANCH
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo $DIR
echo $GOPATH
pwd
#echo "$DIR/../:/go/src/github.com/harmony-one/harmony"
docker build --progress=plain -t harmonyone/localnet-test -f Tests.Dockerfile .

#docker run -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -n
docker run harmonyone/localnet-test -n