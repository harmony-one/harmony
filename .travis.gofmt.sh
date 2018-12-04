#!/bin/bash

if [ -n "$(gofmt -l .)" -o $(golint ./... | wc | awk '{print $1}') -gt "2" ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
else
	echo "Go code is well formatted ;)"
fi
