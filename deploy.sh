#!/bin/bash

set -eo pipefail

function cleanup() {
   for pid in `/bin/ps -fu $USER| grep "benchmark\|txgen\|soldier\|commander\|profiler\|beacon" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
   do
       echo 'Killed process: '$pid
       kill -9 $pid 2> /dev/null
   done
}

function killnode() {
   local port=$1

   if [ -n "port" ]; then
      pid=$(/bin/ps -fu $USER | grep "benchmark" | grep "$port" | awk '{print $2}')
      echo "killing node with port: $port"
      kill -9 $pid 2> /dev/null
      echo "node with port: $port is killed"
   fi
}

trap cleanup SIGINT SIGTERM

function usage {
   local ME=$(basename $0)

   cat<<EOU
USAGE: $ME [OPTIONS] config_file_name

   -h             print this help message
   -p             use peer discovery (default: $PEER)
   -d             enable db support (default: $DB)
   -t             toggle txgen (default: $TXGEN)
   -D duration    txgen run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -k nodeport    kill the node with specified port number (default: $KILLPORT)

This script will build all the binaries and start benchmark and txgen based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

PEER=-peer_discovery
DB=
TXGEN=false
DURATION=90
MIN=5
SHARDS=2
KILLPORT=9004

while getopts "hpdtD:m:s:k:" option; do
   case $option in
      h) usage ;;
      p) PEER='-peer_discovery' ;;
      d) DB='-db_supported' ;;
      t) TXGEN=$OPTARG ;;
      D) DURATION=$OPTARG ;;
      m) MIN=$OPTARG ;;
      s) SHARDS=$OPTARG ;;
      k) KILLPORT=$OPTARG ;;
   esac
done

shift $((OPTIND-1))

config=$1
if [ -z "$config" ]; then
   usage
fi


# Kill nodes if any
cleanup

# Since `go run` will generate a temporary exe every time,
# On windows, your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
echo "compiling ..."
go build -o bin/benchmark
go build -o bin/txgen client/txgen/main.go
go build -o bin/beacon beaconchain/main/main.go

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder

if [ -n "$PEER" ]; then
   echo "launching beacon chain ..."
   ./bin/beacon -numShards $SHARDS > $log_folder/beacon.log 2>&1 &
   sleep 1 #wait or beachchain up
fi

# Start nodes
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode shardID <<< $line
	#echo $ip $port $mode
  if [ "$mode" != "client" ]; then
    if [ -z "$PEER" ]; then
      ./bin/benchmark -ip $ip -port $port -config_file $config -log_folder $log_folder $DB -min_peers $MIN &
    else
      ./bin/benchmark -ip $ip -port $port -log_folder $log_folder $DB $PEER -min_peers $MIN &
      sleep 1
    fi
  fi
done < $config

# Emulate node offline
(sleep 45; killnode $KILLPORT) &

echo "launching txgen ..."
if [ "$TXGEN" == "true" ]; then
   echo "launching txgen ..."
   if [ -z "$PEER" ]; then
      ./bin/txgen -config_file $config -log_folder $log_folder -duration $DURATION
   else
      ./bin/txgen -log_folder $log_folder -duration $DURATION $PEER
   fi
fi

# cleanup
