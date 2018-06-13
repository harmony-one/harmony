#!/bin/bash -x
sudo yum update -y
sudo yum install -y golang

. aws-scripts/setup_golang.sh