#!/usr/bin/env bash
GOOS=linux
GOARCH=amd64
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/benchmark benchmark.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/soldier aws-experiment-launch/experiment/soldier/main.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
env GOOS=$GOOS GOARCH=$GOARCH go build -o bin/txgen client/txgen/main.go
