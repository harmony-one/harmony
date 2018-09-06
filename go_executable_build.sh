#!/usr/bin/env bash

GOOS=linux
GOARCH=amd64

VERSION=$(git rev-list --all --count)
COMMIT=$(git describe --always --long --dirty)
BUILTAT=$(date +%FT%T%z)
BUILTBY=${USER}@

env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o bin/benchmark benchmark.go
env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-X main.version=v${VERSION} -X main.commit=${COMMIT} -X main.builtAt=${BUILTAT} -X main.builtBy=${BUILTBY}" -o bin/txgen client/txgen/main.go

AWSCLI=aws
if [ "$1" != "" ]; then
   AWSCLI+=" --profile $1"
fi

$AWSCLI s3 cp bin/benchmark s3://unique-bucket-bin/benchmark --acl public-read-write
$AWSCLI s3 cp bin/txgen s3://unique-bucket-bin/txgen --acl public-read-write
