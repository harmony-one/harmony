#!/usr/bin/env bash

GOOS=linux
GOARCH=amd64
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/benchmark benchmark.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/txgen client/txgen/main.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/profiler profiler/main.go

AWSCLI=aws
if [ "$1" != "" ]; then
   AWSCLI+=" --profile $1"
fi

$AWSCLI s3 cp bin/benchmark s3://unique-bucket-bin/benchmark --acl public-read-write
$AWSCLI s3 cp bin/txgen s3://unique-bucket-bin/txgen --acl public-read-write
$AWSCLI s3 cp bin/profiler s3://unique-bucket-bin/profiler --acl public-read-write
