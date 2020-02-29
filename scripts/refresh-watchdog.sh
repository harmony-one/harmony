#!/bin/bash

set -eu

ssh devops -- \
    cat /home/ec2-user/edgar/experiment-deploy/pipeline/logs/ds/shard0.txt > shard0.txt

ssh devops -- \
    cat /home/ec2-user/edgar/experiment-deploy/pipeline/logs/ds/shard1.txt > shard1.txt

ssh devops -- \
    cat /home/ec2-user/edgar/experiment-deploy/pipeline/logs/ds/shard2.txt > shard2.txt

scp shard0.txt watchdog:/home/ec2-user/doublesign

scp shard1.txt watchdog:/home/ec2-user/doublesign

scp shard2.txt watchdog:/home/ec2-user/doublesign

ssh watchdog -- \
    sudo systemctl restart harmony-watchdogd@doublesign.service 
