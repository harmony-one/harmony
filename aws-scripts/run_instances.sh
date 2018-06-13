#!/bin/bash -x
cd /home/ec2-user/projects/src/harmony-benchmark
# GOROOT is the location where Go package is installed on your system
source ~/.bash_profile

./deploy_linux.sh local_iplist.txt
./send_txn.sh