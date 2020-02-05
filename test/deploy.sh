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

function cleanup() {
   "${progdir}/kill_node.sh"
}

function cleanup_and_result() {
   "${ROOT}/test/kill_node.sh" 2> /dev/null
}

trap cleanup_and_result SIGINT SIGTERM

function usage {
   local ME=$(basename $0)

   cat<<EOU
USAGE: $ME [OPTIONS] config_file_name [extra args to node]

   -h             print this help message
   -D duration    test run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -n             dryrun mode (default: $DRYRUN)
   -N network     network type (default: $NETWORK)
   -B             don't build the binary

This script will build all the binaries and start harmony based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

DEFAULT_DURATION_NOSYNC=60
DEFAULT_DURATION_SYNC=200

DURATION=
MIN=3
SHARDS=2
DRYRUN=
SYNC=true
NETWORK=localnet
NUM_TEST=10

while getopts "htD:m:s:nBN:" option; do
   case $option in
      h) usage ;;
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
   scripts/go_executable_build.sh
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

sleep $DURATION

cleanup_and_result
