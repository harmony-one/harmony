#!/bin/bash

if [ $(golint ./... | wc | awk '{print $1}') -gt 2 ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
fi
if [ $(./test/deploy.sh ./test/configs/local_config1.txt 2>&1  | grep "HOORAY" | wc | awk '{print $1}') -lt 10 ]; then
    echo "The code did not produce enough consensus."
    exit 1
fi
if [ -n "$(gofmt -l .)" ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
else
	echo "Go code is well formatted ;)"
fi
