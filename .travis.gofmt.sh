#!/bin/bash

# Check golint.
if [ $(golint ./... | wc -l) -gt 2 ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
fi
# Run deploy.sh and count how many times HOORAY appearing in the output. If it does not produce enough the submission may cause the consensus.
if [ $(./test/deploy.sh -D 30 ./test/configs/local_config1.txt 2>&1  | grep "HOORAY" | wc -l) -lt 10 ]; then
    echo "The code did not produce enough consensus."
    exit 1
fi
# Run kill nodes in case deploy.sh got broken and did not kill all nodes.
./test/kill_node.sh
# Check gofmt.
if [ -n "$(gofmt -l .)" ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
else
	echo "Go code is well formatted ;)"
fi
