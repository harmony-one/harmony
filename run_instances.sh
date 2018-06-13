#!/bin/bash -x
cd /home/ec2-user/
./deploy.sh local_iplist.txt
./send_txn.sh