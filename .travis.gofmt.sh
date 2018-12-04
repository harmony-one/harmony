#!/bin/bash

golint ./...
echo $(golint ./... | wc | awk '{print $1}')

if [ $(golint ./... | wc | awk '{print $1}') -gt 2 ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
fi
if [ -n "$(gofmt -l .)" ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
else
	echo "Go code is well formatted ;)"
fi
