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
   "${ROOT}/test/kill_node.sh" 2> /dev/null || true
}

function debug_staking() {
   source "$(go env GOPATH)/src/github.com/harmony-one/harmony/scripts/setup_bls_build_flags.sh"
   hmy_gosdk="$(go env GOPATH)/src/github.com/harmony-one/go-sdk"
   hmy_bin="${hmy_gosdk}/hmy"
   hmy_ops="/tmp/harmony-ops"
   keystore="${hmy_ops}/test-automation/api-tests/LocalnetValidatorKeys"

   rm -rf $hmy_ops
   git clone https://github.com/harmony-one/harmony-ops.git $hmy_ops

   if [ ! -d "${hmy_gosdk}" ]; then
     git clone https://github.com/harmony-one/go-sdk.git $hmy_gosdk
   fi
   if [ ! -f "${hmy_bin}" ]; then
     make -C $hmy_gosdk
   fi

   hmy_version=$($hmy_bin version 2>&1 >/dev/null)
   if [[ "${hmy_version:32:3}" -lt "135" ]]; then
     echo "Aborting staking tests since CLI version is out of date."
     return
   fi

   python3 -m pip install pyhmy
   python3 -m pip install requests
   python3 "${hmy_ops}/test-automation/api-tests/test.py" --keystore $keystore \
            --cli_path $hmy_bin --test_dir "${hmy_ops}/test-automation/api-tests/tests/"  \
            --rpc_endpoint_src="http://localhost:9500/" --rpc_endpoint_dst="http://localhost:9501/" --ignore_regression_test
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

This script will build all the binaries and start harmony and based on the configuration file.

EXAMPLES:

   $ME local_config.txt
   $ME -p local_config.txt

EOU
   exit 0
}

DURATION=60000
MIN=3
SHARDS=2
DRYRUN=
SYNC=true
NETWORK=localnet

while getopts "hD:m:s:nBN:" option; do
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

# Kill nodes if any
cleanup

# Since `go run` will generate a temporary exe every time,
# On windows, your system will pop up a network security dialog for each instance
# and you won't be able to turn it off. With `go build` generating one
# exe, the dialog will only pop up once at the very first time.
# Also it's recommended to use `go build` for testing the whole exe. 
if [ "${NOBUILD}" != "true" ]; then
   pushd $ROOT
   scripts/go_executable_build.sh -S
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
