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
   for pid in `/bin/ps -fu $USER| grep "harmony\|soldier\|commander\|profiler\|beacon\|bootnode" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
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
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -k nodeport    kill the node with specified port number (default: $KILLPORT)
   -n             dryrun mode (default: $DRYRUN)
   -S             enable sync test (default: $SYNC)

This script will build all the binaries and start harmony and based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

DB=
MIN=5
SHARDS=2
KILLPORT=9004
SYNC=true
DRYRUN=

while getopts "hd:m:s:k:nSP" option; do
   case $option in
      h) usage ;;
      d) DB='-db_supported' ;;
      m) MIN=$OPTARG ;;
      s) SHARDS=$OPTARG ;;
      k) KILLPORT=$OPTARG ;;
      n) DRYRUN=echo ;;
      S) SYNC=true ;;
   esac
done

shift $((OPTIND-1))

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

HMY_OPT=
# Change to the beacon chain output from deploy.sh
HMY_OPT2=
HMY_OPT3=

unset -v latest_bootnode_log
latest_bootnode_log=$(ls -tr "${ROOT}"/tmp_log/log-*/bootnode.log | tail -1)
case "${latest_bootnode_log}" in
"")
	echo "cannot determine latest bootnode log"
	exit 69
	;;
esac
unset -v bn_ma
bn_ma=$(sed -n 's:^.*BN_MA=::p' "${latest_bootnode_log}" | tail -1)
case "${bn_ma}" in
"")
	echo "cannot determine boot node address from ${latest_bootnode_log}"
	exit 69
	;;
esac
echo "autodetected boot node multiaddr: ${bn_ma}"
HMY_OPT2="-bootnodes ${bn_ma}"

for i in 0{1..5} # {10..99}
do
    echo "launching new node $i ..."
    ($DRYRUN $ROOT/bin/harmony -ip 127.0.0.1 -port 91$i -log_folder $log_folder -is_newnode $DB -account_index $i -min_peers $MIN $HMY_OPT $HMY_OPT2 $HMY_OPT3 -key /tmp/127.0.0.1-91$i.key 2>&1 | tee -a $LOG_FILE ) &
    sleep 5
done
