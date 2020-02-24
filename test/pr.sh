#!/usr/bin/env bash
pkill -9 '^(harmony|txgen|soldier|commander|profiler|beacon|bootnode)$' | sed 's/^/Killed process: /'
rm -rf db-127.0.0.1-*
docker run -v "$(go env GOPATH)/src/github.com/harmony-one/harmony:/root/go/src/github.com/harmony-one/harmony" -it harmonyone/test-pr $@