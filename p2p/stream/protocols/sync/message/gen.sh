#!/bin/bash

docker run --platform linux/amd64 -v ${PWD}:/tmp ${PROTOC_IMAGE} /tmp/msg.proto