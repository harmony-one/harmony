#!/bin/bash

# used versions
#go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26
#go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1
#SRC_DIR=$(dirname $0)
#protoc -I ${SRC_DIR}/proto/ ${SRC_DIR}/proto/downloader.proto --go_out=${SRC_DIR}/proto --go-grpc_out=${SRC_DIR}/proto

docker run --platform linux/amd64 -v ${PWD}:/tmp ${PROTOC_IMAGE}  /tmp/downloader.proto
