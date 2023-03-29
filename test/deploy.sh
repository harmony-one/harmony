#!/usr/bin/env bash
set -eo pipefail

unset -v progdir
case "${0}" in
*/*) progdir="${0%/*}" ;;
*) progdir=. ;;
esac

ROOT="${progdir}/.."
USER=$(whoami)
OS=$(uname -s)

. "${ROOT}/scripts/setup_bls_build_flags.sh"

function cleanup() {
  "${progdir}/kill_node.sh"
}

function build() {
  if [[ "${NOBUILD}" != "true" ]]; then
    pushd ${ROOT}
    export GO111MODULE=on
    if [[ "$OS" == "Darwin" ]]; then
      # MacOS doesn't support static build
      scripts/go_executable_build.sh -S
    else
      # Static build on Linux platform
      scripts/go_executable_build.sh -s
    fi
    popd
  fi
}

function setup() {
  # Setup blspass file
  mkdir -p ${ROOT}/.hmy
  if [[ ! -f "${ROOT}/.hmy/blspass.txt" ]]; then
    touch "${ROOT}/.hmy/blspass.txt"
  fi

  # Kill nodes if any
  cleanup

  # Note that the binarys only works on MacOS & Linux
  build

  # Create a tmp folder for logs
  t=$(date +"%Y%m%d-%H%M%S")
  log_folder="${ROOT}/tmp_log/log-$t"
  mkdir -p "${log_folder}"
  LOG_FILE=${log_folder}/r.log
}

function launch_bootnode() {
  echo "launching boot node ..."
  ${DRYRUN} ${ROOT}/bin/bootnode -port 19876 -max_conn_per_ip 100 -force_public true >"${log_folder}"/bootnode.log 2>&1 | tee -a "${LOG_FILE}" &
  sleep 1
  BN_MA=$(grep "BN_MA" "${log_folder}"/bootnode.log | awk -F\= ' { print $2 } ')
  echo "bootnode launched." + " $BN_MA"
}

function launch_localnet() {
  launch_bootnode

  unset -v base_args
  declare -a base_args args

  if ${VERBOSE}; then
    verbosity=5
  else
    verbosity=3
  fi

  base_args=(--log_folder "${log_folder}" --min_peers "${MIN}" --bootnodes "${BN_MA}" "--network_type=$NETWORK" --blspass file:"${ROOT}/.hmy/blspass.txt" "--dns=false" "--verbosity=${verbosity}" "--p2p.security.max-conn-per-ip=100")
  sleep 2

  # Start nodes
  i=-1
  while IFS='' read -r line || [[ -n "$line" ]]; do
    i=$((i + 1))

    # Read config for i-th node form config file
    IFS=' ' read -r ip port mode bls_key shard node_config <<<"${line}"
    args=("${base_args[@]}" --ip "${ip}" --port "${port}" --key "/tmp/${ip}-${port}.key" --db_dir "${ROOT}/db-${ip}-${port}" "--broadcast_invalid_tx=false")
    if [[ -z "$ip" || -z "$port" || "$ip" == "#" ]]; then
      echo "skip empty line or node or comment"
      continue
    fi
    if [[ $EXPOSEAPIS == "true" ]]; then
      args=("${args[@]}" "--http.ip=0.0.0.0" "--ws.ip=0.0.0.0")
    fi

    # Setup BLS key for i-th localnet node
    if [[ ! -e "$bls_key" ]]; then
      args=("${args[@]}" --blskey_file "BLSKEY")
    elif [[ -f "$bls_key" ]]; then
      args=("${args[@]}" --blskey_file "${ROOT}/${bls_key}")
    elif [[ -d "$bls_key" ]]; then
      args=("${args[@]}" --blsfolder "${ROOT}/${bls_key}")
    else
      echo "skipping unknown node"
      continue
    fi

    # Setup node config for i-th localnet node
    if [[ -f "$node_config" ]]; then
      echo "node ${i} configuration is loaded from: ${node_config}"
      args=("${args[@]}" --config "${node_config}")
    fi

    # Setup flags for i-th node based on config
    case "${mode}" in
    explorer)
      args=("${args[@]}" "--node_type=explorer" "--shard_id=${shard}" "--http.rosetta=true" "--run.archive")
      ;;
    archival)
      args=("${args[@]}" --is_archival --run.legacy)
      ;;
    leader)
      args=("${args[@]}" --is_leader --run.legacy)
      ;;
    external)
      ;;
    client)
      args=("${args[@]}" --run.legacy)
      ;;
    validator)
      args=("${args[@]}" --run.legacy)
      ;;
    esac

    # Start the node
    ${DRYRUN} "${ROOT}/bin/harmony" "${args[@]}" "${extra_args[@]}" 2>&1 | tee -a "${LOG_FILE}" &
  done <"${config}"
}

trap cleanup SIGINT SIGTERM

function usage() {
  local ME=$(basename $0)

  echo "
USAGE: $ME [OPTIONS] config_file_name [extra args to node]

   -h             print this help message
   -D duration    test run duration (default: $DURATION)
   -m min_peers   minimal number of peers to start consensus (default: $MIN)
   -s shards      number of shards (default: $SHARDS)
   -n             dryrun mode (default: $DRYRUN)
   -N network     network type (default: $NETWORK)
   -B             don't build the binary
   -v             verbosity in log (default: $VERBOSE)
   -e             expose WS & HTTP ip (default: $EXPOSEAPIS)

This script will build all the binaries and start harmony and based on the configuration file.

EXAMPLES:

   $ME local_config.txt
"
  exit 0
}

DURATION=60000
MIN=4
SHARDS=2
DRYRUN=
NETWORK=localnet
VERBOSE=false
NOBUILD=false
EXPOSEAPIS=false

while getopts "hD:m:s:nBN:ve" option; do
  case ${option} in
  h) usage ;;
  D) DURATION=$OPTARG ;;
  m) MIN=$OPTARG ;;
  s) SHARDS=$OPTARG ;;
  n) DRYRUN=echo ;;
  B) NOBUILD=true ;;
  N) NETWORK=$OPTARG ;;
  v) VERBOSE=true ;;
  e) EXPOSEAPIS=true ;;
  *) usage ;;
  esac
done

shift $((OPTIND - 1))

config=$1
shift 1 || usage
unset -v extra_args
declare -a extra_args
extra_args=("$@")

setup
launch_localnet
sleep "${DURATION}"
cleanup || true
