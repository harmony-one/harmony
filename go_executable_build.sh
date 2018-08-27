#!/usr/bin/env bash

GOOS=linux
GOARCH=amd64
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/benchmark benchmark.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/txgen client/txgen/main.go

AWSCLI=aws
if [ "$1" != "" ]; then
   AWSCLI+=" --profile $1"
fi

$AWSCLI s3 cp bin/benchmark s3://unique-bucket-bin/benchmark --acl public-read-write
$AWSCLI s3 cp bin/soldier s3://unique-bucket-bin/soldier --acl public-read-write
$AWSCLI s3 cp bin/commander s3://unique-bucket-bin/commander --acl public-read-write
$AWSCLI s3 cp bin/txgen s3://unique-bucket-bin/txgen --acl public-read-write
$AWSCLI s3 cp kill_node.sh s3://unique-bucket-bin/kill_node.sh --acl public-read-write
