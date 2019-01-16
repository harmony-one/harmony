#!/bin/bash
# sync test

ROOT=$(dirname $0)/..
ip=127.0.0.1
port=9022

t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

$ROOT/bin/harmony -ip $ip -port $port -log_folder $log_folder  -min_peers 5 2>&1 | tee -a $LOG_FILE &
