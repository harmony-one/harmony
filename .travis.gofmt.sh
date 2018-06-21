#!/bin/bash

if [ -n "$(gofmt -l .)" ]; then
    echo "Go code is not formatted:"
    gofmt -d .
    exit 1
else
	echo "Go code is well formatted ;)"
fi