#!/bin/bash

docker run -v ${PWD}:/tmp ${PROTOC_IMAGE}  /tmp/harmonymessage.proto
