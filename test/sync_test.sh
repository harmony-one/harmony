#!/bin/bash
# sync test
# example:  ./sync_test.sh 9022 5    will add 5 node with port number begin from 9023 to 9027

port=$1
num=$2

ROOT=$(dirname $0)/..
ip=127.0.0.1

t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

for i in `seq 1 $num` ; do
    PORT=$(( $port + $i )) 
    $ROOT/bin/harmony -ip $ip -port $PORT -log_folder $log_folder  -min_peers 5 2>&1 | tee -a $LOG_FILE &
done

  #echo `expr $port + $i`
