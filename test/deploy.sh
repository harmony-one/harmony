#!/bin/bash

unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}" ;;
*) progdir=.  ;;
esac

ROOT="${progdir}/.."
USER=$(whoami)

. "${ROOT}/scripts/setup_bls_build_flags.sh"

# set -x
set -eo pipefail

export GO111MODULE=on

mkdir -p .hmy
if [ -f ".hmy/blspass.txt" ]
then
   echo ".hmy/blspass.txt already in local."
else
   touch .hmy/blspass.txt
fi

function check_result() {
   err=false

   echo "====== WALLET BALANCES ======" > $RESULT_FILE
   $ROOT/bin/wallet -p local balances --address $ACC1  >> $RESULT_FILE
   $ROOT/bin/wallet -p local balances --address $ACC2  >> $RESULT_FILE
   $ROOT/bin/wallet -p local balances --address $ACC3  >> $RESULT_FILE
   echo "====== RESULTS ======" >> $RESULT_FILE

   TEST_ACC1=$($ROOT/bin/wallet -p local balances --address $ACC1 | grep 'Shard 0' | grep -oE 'nonce:.[0-9]+' | awk ' { print $2 } ')
   TEST_ACC2=$($ROOT/bin/wallet -p local balances --address $ACC2 | grep 'Shard 1' | grep -oE 'nonce:.[0-9]+' | awk ' { print $2 } ')
   BAL0_ACC3=$($ROOT/bin/wallet -p local balances --address $ACC3 | grep 'Shard 0' | grep -oE '[0-9]\.[0-9]+,' | awk -F\. ' { print $1 } ')
   BAL1_ACC3=$($ROOT/bin/wallet -p local balances --address $ACC3 | grep 'Shard 1' | grep -oE '[0-9]\.[0-9]+,' | awk -F\. ' { print $1 } ')

   if [[ $TEST_ACC1 -ne $NUM_TEST || $TEST_ACC2 -ne $NUM_TEST ]]; then
      echo -e "FAIL number of nonce. Expected Result: $NUM_TEST.\nAccount1:$TEST_ACC1\nAccount2:$TEST_ACC2\n" >> $RESULT_FILE
      err=true
   fi
   if [[ $BAL0_ACC3 -ne 1 || $BAL1_ACC3 -ne 1 ]]; then
      echo "FAIL balance of $ACC3. Expected Result: 1.\nShard0:$BAL0_ACC3\nShard1:$BAL1_ACC3\n" >> $RESULT_FILE
      err=true
   fi

   $err || echo "PASS" >> $RESULT_FILE
}

function cleanup() {
   "${progdir}/kill_node.sh"
}

function cleanup_and_result() {
   "${ROOT}/test/kill_node.sh" 2> /dev/null
   [ -e $RESULT_FILE ] && cat $RESULT_FILE
}

trap cleanup_and_result SIGINT SIGTERM

function usage {
   local ME=$(basename $0)

   cat<<EOU
USAGE: $ME [OPTIONS] config_file_name [extra args to node]

   -h             print this help message
   -t             disable wallet test (default: $DOTEST)
   -D duration    test run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -n             dryrun mode (default: $DRYRUN)
   -N network     network type (default: $NETWORK)
   -B             don't build the binary

This script will build all the binaries and start harmony and txgen based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

DEFAULT_DURATION_NOSYNC=60
DEFAULT_DURATION_SYNC=200

DOTEST=true
DURATION=
MIN=3
SHARDS=2
DRYRUN=
SYNC=true
NETWORK=localnet
NUM_TEST=10
ACC1=one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj
ACC2=one1uyshu2jgv8w465yc8kkny36thlt2wvel89tcmg
ACC3=one1r4zyyjqrulf935a479sgqlpa78kz7zlcg2jfen

while getopts "htD:m:s:nBN:" option; do
   case $option in
      h) usage ;;
      t) DOTEST=false ;;
      D) DURATION=$OPTARG ;;
      m) MIN=$OPTARG ;;
      s) SHARDS=$OPTARG ;;
      n) DRYRUN=echo ;;
      B) NOBUILD=true ;;
      N) NETWORK=$OPTARG ;;
   esac
done

shift $((OPTIND-1))

config=$1
shift 1 || usage
unset -v extra_args
declare -a extra_args
extra_args=("$@")

case "${DURATION-}" in
"")
    case "${SYNC}" in
    false) DURATION="${DEFAULT_DURATION_NOSYNC}";;
    true) DURATION="${DEFAULT_DURATION_SYNC}";;
    esac
    ;;
esac

# Kill nodes if any
cleanup

# Since `go run` will generate a temporary exe every time,
# On windows, your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
if [ "${NOBUILD}" != "true" ]; then
   pushd $ROOT
   scripts/go_executable_build.sh harmony
   scripts/go_executable_build.sh wallet
   scripts/go_executable_build.sh bootnode
   popd
fi

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log
RESULT_FILE=$log_folder/result.txt

echo "launching boot node ..."
$DRYRUN $ROOT/bin/bootnode -port 19876 > $log_folder/bootnode.log 2>&1 | tee -a $LOG_FILE &
sleep 1
BN_MA=$(grep "BN_MA" $log_folder/bootnode.log | awk -F\= ' { print $2 } ')
echo "bootnode launched." + " $BN_MA"

unset -v base_args
declare -a base_args args
base_args=(-log_folder "${log_folder}" -min_peers "${MIN}" -bootnodes "${BN_MA}" -network_type="$NETWORK" -blspass file:.hmy/blspass.txt -dns=false)
sleep 2

# Start nodes
i=0
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode account blspub <<< $line
  args=("${base_args[@]}" -ip "${ip}" -port "${port}" -key "/tmp/${ip}-${port}.key" -db_dir "db-${ip}-${port}")
  if [[ -z "$ip" || -z "$port" ]]; then
     echo "skip empty node"
     continue
  fi

  if [ ! -e .hmy/${blspub}.key ]; then
    args=("${args[@]}" -blskey_file "BLSKEY")
  else
    args=("${args[@]}" -blskey_file ".hmy/${blspub}.key")
  fi

  case "${mode}" in leader*) args=("${args[@]}" -is_leader);; esac
  case "${mode}" in *archival|archival) args=("${args[@]}" -is_archival);; esac
  case "${mode}" in explorer*) args=("${args[@]}" -node_type=explorer -shard_id=0);; esac
  case "${mode}" in
  client) ;;
  *) $DRYRUN "${ROOT}/bin/harmony" "${args[@]}" "${extra_args[@]}" 2>&1 | tee -a "${LOG_FILE}" &;;
  esac
  i=$((i+1))
done < $config

if [ "$DOTEST" == "true" ]; then
   echo "waiting for some block rewards"
   sleep 60
   i=1
   echo "launching wallet cross shard transfer test"
   while [ $i -le $NUM_TEST ]; do
      "${ROOT}/bin/wallet" -p local transfer --from $ACC1 --to $ACC3 --shardID 0 --toShardID 1 --amount 0.1 --pass pass:"" 2>&1 | tee -a "${LOG_FILE}"
      sleep 20
      "${ROOT}/bin/wallet" -p local transfer --from $ACC2 --to $ACC3 --shardID 1 --toShardID 0 --amount 0.1 --pass pass:"" 2>&1 | tee -a "${LOG_FILE}"
      sleep 20
      i=$((i+1))
   done
   echo "waiting for the result"
   sleep 20
   check_result
   [ -e $RESULT_FILE ] && cat $RESULT_FILE
fi

sleep $DURATION

cleanup_and_result
