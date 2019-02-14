#!/bin/bash

ROOT=$(dirname $0)/..
USER=$(whoami)

. "${ROOT}/scripts/setup_bls_build_flags.sh"

set -x
set -eo pipefail

function check_result() {
   find $log_folder -name leader-*.log > $log_folder/all-leaders.txt
   find $log_folder -name validator-*.log > $log_folder/all-validators.txt

   echo ====== RESULTS ======
   results=$($ROOT/test/cal_tps.sh $log_folder/all-leaders.txt $log_folder/all-validators.txt)
   echo $results | tee -a $LOG_FILE
   echo $results > $log_folder/tps.log
}

function cleanup() {
   for pid in `/bin/ps -fu $USER| grep "harmony\|txgen\|soldier\|commander\|profiler\|beacon\|bootnode" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
   do
       echo 'Killed process: '$pid
       $DRYRUN kill -9 $pid 2> /dev/null
   done
   # Remove bc_config.json before starting experiment.
   rm -f bc_config.json
   rm -rf ./db/harmony_*
}

function killnode() {
   local port=$1

   if [ -n "port" ]; then
      pid=$(/bin/ps -fu $USER | grep "harmony" | grep "$port" | awk '{print $2}')
      echo "killing node with port: $port"
      $DRYRUN kill -9 $pid 2> /dev/null
      echo "node with port: $port is killed"
   fi
}

trap cleanup SIGINT SIGTERM

function usage {
   local ME=$(basename $0)

   cat<<EOU
USAGE: $ME [OPTIONS] config_file_name

   -h             print this help message
   -d             enable db support (default: $DB)
   -t             toggle txgen (default: $TXGEN)
   -D duration    txgen run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -k nodeport    kill the node with specified port number (default: $KILLPORT)
   -n             dryrun mode (default: $DRYRUN)
   -S             enable sync test (default: $SYNC)
   -P             enable libp2p peer discovery test (default: $P2P)

This script will build all the binaries and start harmony and txgen based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

DB=
TXGEN=true
DURATION=90
MIN=2
SHARDS=2
KILLPORT=9004
SYNC=false
DRYRUN=
P2P=false

while getopts "hdtD:m:s:k:nSP" option; do
   case $option in
      h) usage ;;
      d) DB='-db_supported' ;;
      t) TXGEN=false ;;
      D) DURATION=$OPTARG ;;
      m) MIN=$OPTARG ;;
      s) SHARDS=$OPTARG ;;
      k) KILLPORT=$OPTARG ;;
      n) DRYRUN=echo ;;
      S) SYNC=true ;;
      P) P2P=true ;;
   esac
done

shift $((OPTIND-1))

config=$1
if [ -z "$config" ]; then
   usage
fi

if [ "$SYNC" == "true" ]; then
    DURATION=300
    SHARDS=1
fi

# Kill nodes if any
cleanup

# Since `go run` will generate a temporary exe every time,
# On windows, your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
pushd $ROOT
echo "compiling ..."
go build -o bin/harmony cmd/harmony.go
go build -o bin/txgen cmd/client/txgen/main.go
go build -o bin/beacon cmd/beaconchain/main.go
go build -o bin/bootnode cmd/bootnode/main.go
popd

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

HMY_OPT=
HMY_OPT2=

if [ "$P2P" == "false" ]; then
   echo "launching beacon chain ..."
   $DRYRUN $ROOT/bin/beacon -numShards $SHARDS > $log_folder/beacon.log 2>&1 | tee -a $LOG_FILE &
   sleep 1 #waiting for beaconchain
   BC_MA=$(grep "Beacon Chain Started" $log_folder/beacon.log | awk -F: ' { print $2 } ')
   HMY_OPT=" -bc_addr $BC_MA"
else
   echo "launching boot node ..."
   $DRYRUN $ROOT/bin/bootnode > $log_folder/bootnode.log 2>&1 | tee -a $LOG_FILE &
   sleep 1
   BN_MA=$(grep "BN_MA" $log_folder/bootnode.log | awk -F\= ' { print $2 } ')
   HMY_OPT2=" -bootnodes $BN_MA"
   HMY_OPT2+=" -libp2p_pd -is_beacon"
   TXGEN=false
fi

NUM_NN=0

# Start nodes
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode shardID <<< $line
  if [ "$mode" == "leader" ]; then
     $DRYRUN $ROOT/bin/harmony -ip $ip -port $port -log_folder $log_folder $DB -min_peers $MIN $HMY_OPT $HMY_OPT2 -key /tmp/$ip-$port.key -is_leader 2>&1 | tee -a $LOG_FILE &
  fi
  if [ "$mode" == "validator" ]; then
     $DRYRUN $ROOT/bin/harmony -ip $ip -port $port -log_folder $log_folder $DB -min_peers $MIN $HMY_OPT $HMY_OPT2 -key /tmp/$ip-$port.key 2>&1 | tee -a $LOG_FILE &
  fi
  sleep 0.5
  if [[ "$mode" == "newnode" && "$SYNC" == "true" ]]; then
     (( NUM_NN += 35 ))
     (sleep $NUM_NN; $DRYRUN $ROOT/bin/harmony -ip $ip -port $port -log_folder $log_folder $DB -min_peers $MIN $HMY_OPT $HMY_OPT2 -key /tmp/$ip-$port.key 2>&1 | tee -a $LOG_FILE ) &
  fi
done < $config

# Emulate node offline
if [ "$SYNC" == "false" ]; then
 (sleep 45; killnode $KILLPORT) &
fi

if [ "$TXGEN" == "true" ]; then
   echo "launching txgen ... wait"
   sleep 2
   line=$(grep client $config)
   IFS=' ' read ip port mode shardID <<< $line
   if [ "$mode" == "client" ]; then
      $DRYRUN $ROOT/bin/txgen -log_folder $log_folder -duration $DURATION -ip $ip -port $port $HMY_OPT 2>&1 | tee -a $LOG_FILE
   fi
else
   sleep $DURATION
fi

# save bc_config.json
[ -e bc_config.json ] && cp -f bc_config.json $log_folder

cleanup
check_result
