#!/bin/bash -x
cd /home/ec2-user/projects/src/harmony-benchmark
go get github.com/go-stack/stack
./deploy_one_instance.sh global_nodes.txt