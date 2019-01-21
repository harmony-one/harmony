#!/bin/bash
# sync test
# example:  ./sync_test.sh 9022     will add n/3 new nodes with port number begin from 9023 

port=$1

ROOT=$(dirname $0)/..
ip=127.0.0.1

t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log


total=`ps |grep harmony | wc -l |awk '{print $1}'`
num=`perl -e "print $total/3"`
opt=`ps |grep harmony | awk '{print $14}' | head -n1`

echo "total is "$total
echo "number is "$num
echo "-bc_addr option is "$opt

for i in `seq 1 $num` ; do
    PORT=$(( $port + $i )) 
    $ROOT/bin/harmony -ip $ip -port $PORT -log_folder $log_folder -min_peers 5  -bc_addr $opt 2>&1 | tee -a $LOG_FILE &
done
