#!/bin/bash

ROOT=$(dirname $0)/..
USER=$(whoami)
DRYRUN=

set -x
set -eo pipefail

echo "compiling ..."
go build -o bin/harmony cmd/harmony.go
go build -o bin/beacon cmd/beaconchain/main.go


# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log
rm -f bc_config.json

echo "launching beacon chain ..."
$DRYRUN $ROOT/bin/beacon -numShards 3 > $log_folder/beacon.log 2>&1 | tee -a $LOG_FILE &
sleep 2 #waiting for beaconchain
MA=$(grep "Beacon Chain Started" $log_folder/beacon.log | awk -F: ' { print $2 } ')

if [ -n "$MA" ]; then
   HMY_OPT="-bc_addr $MA"
fi

DB='-db_supported'

$ROOT/bin/harmony -ip 127.0.0.1 -port 9000 -log_folder $log_folder $DB -min_peers 0 $HMY_OPT 2>&1 | tee -a $LOG_FILE &
