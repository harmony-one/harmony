#!/bin/bash

go test ./...
if [ $? -ne 0 ]; then
    echo "Some tests failed"
    exit 1
fi
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
