#!/bin/bash

docker run -v ${PWD}:/tmp ${PROTOC_IMAGE} /tmp/msg.proto
