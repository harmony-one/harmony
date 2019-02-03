#!/bin/bash

ROOT=$(dirname $0)/..

. "${ROOT}/scripts/setup_bls_build_flags.sh"

function cleanup() {
   for pid in `/bin/ps -fu $USER| grep "harmony\|txgen\|soldier\|commander\|profiler\|beacon\|bootnode" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
   do
       echo 'Killed process: '$pid
       $DRYRUN kill -9 $pid 2> /dev/null
   done
   rm -rf ./db/harmony_*
}

function usage {
   local ME=$(basename $0)

   cat<<EOU
USAGE: $ME [OPTIONS] config_file_name

   -h             print this help message
 
This script will test the peer discovery.

EXAMPLES:

   $ME test/configs/local_config1.txt
EOU
   exit 1
}

MIN=3

while getopts "h" option; do
   case $option in
      h) usage ;;
   esac
done

shift $((OPTIND-1))
config=$1
if [ -z "$config" ]; then
   usage
fi

trap cleanup SIGINT SIGTERM

go build -o bin/bootnode cmd/bootnode/main.go
go build -o bin/harmony cmd/harmony.go
# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

bin/bootnode 2>&1 | tee -a $LOG_FILE &

sleep 1

while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode shardID <<< $line
  if [[ "$mode" == "leader" || "$mode" == "validator" ]]; then
     $DRYRUN $ROOT/bin/harmony -ip $ip -port $port -log_folder $log_folder -min_peers $MIN -key /tmp/$ip.$port.key 2>&1 | tee -a $LOG_FILE &
     sleep 0.5
  fi
done < $config

echo "sleeping 10s ..."
sleep 10

cleanup
