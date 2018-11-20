#!/bin/bash

DIRROOT=$(dirname $0)

go test ./...
$DIRROOT/.travis.gofmt.sh
