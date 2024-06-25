#!/usr/bin/env bash
set -e

for i in {1..10}
do
	echo "=== START TEST NUMBER ${i} ==="
	docker run -v "$(go env GOPATH)/src/github.com/harmony-one/harmony:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test
	echo "=== END TEST NUMBER ${i} ==="
done
