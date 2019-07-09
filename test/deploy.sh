#!/bin/bash

ROOT=$(dirname $0)/..
USER=$(whoami)

. "${ROOT}/scripts/setup_bls_build_flags.sh"

set -x
set -eo pipefail

export GO111MODULE=on

if [ -f "blspass.txt" ]
then
   echo "blspass.txt already in local."
else
   aws s3 cp s3://harmony-pass/blspass.txt blspass.txt
fi

function check_result() {
   find $log_folder -name leader-*.log > $log_folder/all-leaders.txt
   find $log_folder -name zerolog-validator-*.log > $log_folder/all-validators.txt
   find $log_folder -name archival-*.log >> $log_folder/all-validators.txt

   echo ====== RESULTS ======
   results=$($ROOT/test/cal_tps.sh $log_folder/all-leaders.txt $log_folder/all-validators.txt)
   echo $results | tee -a $LOG_FILE
   echo $results > $log_folder/tps.log
}

function cleanup() {
   for pid in `/bin/ps -fu $USER| grep "harmony\|txgen\|soldier\|commander\|profiler\|bootnode" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
   do
       echo 'Killed process: '$pid
       $DRYRUN kill -9 $pid 2> /dev/null
   done
   rm -rf ./db/harmony_*
   rm -rf ./db*
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
USAGE: $ME [OPTIONS] config_file_name [extra args to node]

   -h             print this help message
   -d             enable db support (default: $DB)
   -t             toggle txgen (default: $TXGEN)
   -D duration    txgen run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -n             dryrun mode (default: $DRYRUN)
   -S             disable sync test (default: $SYNC)
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

DB=false
TXGEN=true
DURATION=
MIN=3
SHARDS=2
SYNC=true
DRYRUN=

while getopts "hdtD:m:s:nSB" option; do
   case $option in
      h) usage ;;
      d) DB=true ;;
      t) TXGEN=false ;;
      D) DURATION=$OPTARG ;;
      m) MIN=$OPTARG ;;
      s) SHARDS=$OPTARG ;;
      n) DRYRUN=echo ;;
      S) SYNC=false ;;
      B) NOBUILD=true ;;
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
   echo "compiling ..."
   go build -o bin/harmony cmd/harmony/main.go
   go build -o bin/txgen cmd/client/txgen/main.go
   go build -o bin/bootnode cmd/bootnode/main.go
   popd
fi

# Create a tmp folder for logs
t=`date +"%Y%m%d-%H%M%S"`
log_folder="tmp_log/log-$t"

mkdir -p $log_folder
LOG_FILE=$log_folder/r.log

echo "launching boot node ..."
$DRYRUN $ROOT/bin/bootnode -port 19876 > $log_folder/bootnode.log 2>&1 | tee -a $LOG_FILE &
sleep 1
BN_MA=$(grep "BN_MA" $log_folder/bootnode.log | awk -F\= ' { print $2 } ')
echo "bootnode launched." + " $BN_MA"

unset -v base_args
declare -a base_args args
base_args=(-log_folder "${log_folder}" -min_peers "${MIN}" -bootnodes "${BN_MA}")
if "${DB}"
then
  base_args=("${base_args[@]}" -db_supported)
fi

NUM_NN=0

sleep 2

mkdir -p .hmy
# Start nodes
i=0
while IFS='' read -r line || [[ -n "$line" ]]; do
  IFS=' ' read ip port mode account blspub <<< $line
  if [ "${mode}" == "explorer" ]
    then
      args=("${base_args[@]}" -ip "${ip}" -port "${port}" -key "/tmp/${ip}-${port}.key" -db_dir "db-${ip}-${port}")
    else
      if [ -f "${blspub}.key" ]
        then
          echo ""${blspub}.key" already in local."
       else
          if [ ! -e .hmy/${blspub}.key ]; then
             aws s3 cp "s3://harmony-secret-keys/bls-dummy/${blspub}.key" .hmy
          fi
      fi

      args=("${base_args[@]}" -ip "${ip}" -port "${port}" -key "/tmp/${ip}-${port}.key" -db_dir "db-${ip}-${port}" -accounts "${account}" -blspass file:blspass.txt -blskey_file ".hmy/${blspub}.key")
  fi

  args=("${base_args[@]}" -ip "${ip}" -port "${port}" -key "/tmp/${ip}-${port}.key" -db_dir "db-${ip}-${port}" -blspass file:blspass.txt -blskey_file ".hmy/${blspub}.key" -dns=false -network_type="testnet")
  case "${mode}" in
  leader*|validator*) args=("${args[@]}" -is_genesis);;
  esac
  case "${mode}" in leader*) args=("${args[@]}" -is_leader);; esac
  case "${mode}" in *archival|archival) args=("${args[@]}" -is_archival);; esac
  case "${mode}" in explorer*) args=("${args[@]}" -is_genesis=false -is_explorer=true -shard_id=0);; esac
  case "${mode}" in
  newnode)
    "${SYNC}" || continue
    sleep "${NUM_NN}"
    NUM_NN=$((${NUM_NN} + 30))
    ;;
  esac
  case "${mode}" in
  client) ;;
  *) $DRYRUN "${ROOT}/bin/harmony" "${args[@]}" "${extra_args[@]}" 2>&1 | tee -a "${LOG_FILE}" &;;
  esac
  i=$((i+1))
done < $config

if [ "$TXGEN" == "true" ]; then
   echo "launching txgen ... wait"
   # sleep 2
   line=$(grep client $config)
   IFS=' ' read ip port mode account <<< $line
   if [ "$mode" == "client" ]; then
      $DRYRUN $ROOT/bin/txgen -log_folder $log_folder -duration $DURATION -ip $ip -port $port -bootnodes "${BN_MA}" > $LOG_FILE 2>&1
   fi
else
   sleep $DURATION
fi


cleanup
check_result
