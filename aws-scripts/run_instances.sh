#!/bin/bash -x
cd /home/ec2-user/projects/src/harmony-benchmark
./deploy.sh local_iplist.txt
./send_txn.sh