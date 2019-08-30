#!/bin/bash

unset -v progname
progname="${0##*/}"

unset -f msg err

msg() {
   case $# in
   [1-9]*)
      echo "${progname}: $*" >&2
      ;;
   esac
}

err() {
   local code
   code="${1}"
   shift 1
   msg "$@"
   exit "${code}"
}

# https://www.linuxjournal.com/content/validating-ip-address-bash-script
function valid_ip()
{
    local  ip=$1
    local  stat=1

    if [[ $ip =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        OIFS=$IFS
        IFS='.'
        ip=($ip)
        IFS=$OIFS
        [[ ${ip[0]} -le 255 && ${ip[1]} -le 255 \
            && ${ip[2]} -le 255 && ${ip[3]} -le 255 ]]
        stat=$?
    fi
    return $stat
}

function myip() {
# get ipv4 address only, right now only support ipv4 addresses
   PUB_IP=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)
   if valid_ip $PUB_IP; then
      msg "public IP address autodetected: $PUB_IP"
   else
      err 1 "NO valid public IP found: $PUB_IP"
   fi
}

function check_root
{
   if [[ $EUID -ne 0 ]]; then
      msg "this script must be run as root to setup environment"
      msg please use \"sudo ${progname}\"
      exit 1
   fi
}

function add_env
{
   filename=$1
   shift
   grep -qxF "$@" $filename || echo "$@" >> $filename
}

function setup_env
{
   check_root

# setup environment variables, may not be nessary
   sysctl -w net.core.somaxconn=1024
   sysctl -w net.core.netdev_max_backlog=65536
   sysctl -w net.ipv4.tcp_tw_reuse=1
   sysctl -w net.ipv4.tcp_rmem='4096 65536 16777216'
   sysctl -w net.ipv4.tcp_wmem='4096 65536 16777216'
   sysctl -w net.ipv4.tcp_mem='65536 131072 262144'

   add_env /etc/security/limits.conf "* soft     nproc          65535"
   add_env /etc/security/limits.conf "* hard     nproc          65535"
   add_env /etc/security/limits.conf "* soft     nofile         65535"
   add_env /etc/security/limits.conf "* hard     nofile         65535"
   add_env /etc/security/limits.conf "root soft     nproc          65535"
   add_env /etc/security/limits.conf "root hard     nproc          65535"
   add_env /etc/security/limits.conf "root soft     nofile         65535"
   add_env /etc/security/limits.conf "root hard     nofile         65535"
   add_env /etc/pam.d/common-session "session required pam_limits.so"
}

######## main #########
print_usage() {
   cat <<- ENDEND

usage: ${progname} [-1ch] [-k KEYFILE]
   -c             back up database/logs and start clean
                  (use only when directed by Harmony)
   -1             do not loop; run once and exit
   -h             print this help and exit
   -k KEYFILE     use the given BLS key file (default: autodetect)
   -s             run setup env only (must run as root)
   -S             run the ${progname} as non-root user (default: run as root)
   -p passfile    use the given BLS passphrase file
   -D             do not download Harmony binaries (default: download when start)
   -m             collect and upload node metrics to harmony prometheus + grafana
   -N network     join the given network (main, beta, pangaea; default: main)
   -t             equivalent to -N pangaea (deprecated)

example:

   ${progname} -S -k mybls.key

ENDEND
}

usage() {
   msg "$@"
   print_usage >&2
   exit 64  # EX_USAGE
}

unset start_clean loop run_as_root blspass do_not_download metrics network
start_clean=false
loop=true
run_as_root=true
do_not_download=false
metrics=false
network=main
${BLSKEYFILE=}

unset OPTIND OPTARG opt
OPTIND=1
while getopts :1chk:sSp:DmN:t opt
do
   case "${opt}" in
   '?') usage "unrecognized option -${OPTARG}";;
   ':') usage "missing argument for -${OPTARG}";;
   c) start_clean=true;;
   1) loop=false;;
   h) print_usage; exit 0;;
   k) BLSKEYFILE="${OPTARG}";;
   s) setup_env; exit 0;;
   S) run_as_root=false ;;
   p) blspass="${OPTARG}";;
   D) do_not_download=true;;
   m) metrics=true;;
   N) network="${OPTARG}";;
   t) network=pangaea;;
   *) err 70 "unhandled option -${OPTARG}";;  # EX_SOFTWARE
   esac
done
shift $((${OPTIND} - 1))

unset -v bootnodes REL network_type dns_zone

case "${network}" in
main)
  bootnodes=(
    /ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
    /ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
    /ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
  )
  REL=s3
  network_type=mainnet
  dns_zone=t.hmny.io
  ;;
beta)
  bootnodes=(
    /ip4/54.213.43.194/tcp/9868/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
    /ip4/100.26.90.187/tcp/9868/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/13.113.101.219/tcp/12018/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
  )
  REL=testnet
  network_type=testnet
  dns_zone=
  ;;
pangaea)
  bootnodes=(
    /ip4/54.86.126.90/tcp/9867/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
    /ip4/52.40.84.2/tcp/9867/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
  )
  REL=master
  network_type=pangaea
  dns_zone=pga.hmny.io
  ;;
*)
  err 64 "${network}: invalid network"
  ;;
esac

case $# in
[1-9]*)
   usage "extra arguments at the end ($*)"
   ;;
esac

if ${run_as_root}; then
   check_root
fi

case "${BLSKEYFILE}" in
"")
   unset -v f
   for f in \
      ~/*--????-??-??T??-??-??.*Z--bls_???????????????????????????????????????????????????????????????????????????????????????????????? \
      ~/????????????????????????????????????????????????????????????????????????????????????????????????.key \
      *--????-??-??T??-??-??.*Z--bls_???????????????????????????????????????????????????????????????????????????????????????????????? \
      ????????????????????????????????????????????????????????????????????????????????????????????????.key
   do
      [ -f "${f}" ] || continue
      case "${BLSKEYFILE}" in
      "")
         BLSKEYFILE="${f}"
         ;;
      *)
         [ "${f}" -ef "${BLSKEYFILE}" ] || \
            err 69 "multiple key files found (${f}, ${BLSKEYFILE}); please use -k to specify"
         ;;
      esac
   done
   case "${BLSKEYFILE}" in
   "") err 69 "could not autodetect BLS key file; please use -k to specify";;
   esac
   msg "autodetected BLS key file: ${BLSKEYFILE}"
   ;;
*)
   msg "using manually specified BLS key file: ${BLSKEYFILE}"
   ;;
esac

BUCKET=pub.harmony.one
OS=$(uname -s)

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/darwin-x86_64/$REL/
   BIN=( harmony libbls384_256.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib md5sum.txt )
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/linux-x86_64/$REL/
   BIN=( harmony libbls384_256.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so md5sum.txt )
fi

# clean up old files
for bin in "${BIN[@]}"; do
   ${do_not_download} || rm -f ${bin}
done

any_new_binaries() {
   local outdir
   ${do_not_download} && return 1
   outdir="${1:-.}"
   mkdir -p "${outdir}"
   curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}md5sum.txt -o "${outdir}/md5sum.txt" || return $?
   diff $outdir/md5sum.txt md5sum.txt
   return $?
}

download_binaries() {
   local outdir
   ${do_not_download} && return 0
   outdir="${1:-.}"
   mkdir -p "${outdir}"
   for bin in "${BIN[@]}"; do
      curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o "${outdir}/${bin}" || return $?
   done
   chmod +x "${outdir}/harmony"
   (cd "${outdir}" && exec openssl sha256 "${BIN[@]}") > "${outdir}/harmony-checksums.txt"
}

download_binaries || err 69 "initial node software update failed"

NODE_PORT=9000
PUB_IP=
METRICS=
PUSHGATEWAY_IP=
PUSHGATEWAY_PORT=

if [ "$OS" == "Linux" ]; then
   if ${run_as_root}; then
      setup_env
   fi
# Kill existing soldier/node
   fuser -k -n tcp $NODE_PORT
fi

# find my public ip address
myip

unset -v BN_MA bn
for bn in "${bootnodes[@]}"
do
  BN_MA="${BN_MA+"${BN_MA},"}${bn}"
done

if ${start_clean}
then
   msg "backing up old database/logs (-c)"
   unset -v backup_dir now
   now=$(date -u +%Y-%m-%dT%H:%M:%SZ)
   mkdir -p backups
   backup_dir=$(mktemp -d "backups/${now}.XXXXXX")
   mv harmony_db_* latest "${backup_dir}/" || :
   rm -rf latest
fi
mkdir -p latest

unset -v check_update_pid

cleanup() {
   local trap_sig kill_sig

   trap_sig="${1:-EXIT}"

   kill_sig="${trap_sig}"
   case "${kill_sig}" in
   0|EXIT|2|INT) kill_sig=TERM;;
   esac

   case "${check_update_pid+set}" in
   set)
      msg "terminating update checker (pid ${check_update_pid})"
      kill -${kill_sig} "${check_update_pid}"
      ;;
   esac
}

unset -v trap_sigs trap_sig
trap_sigs="EXIT HUP INT TERM"

trap_func() {
   local trap_sig="${1-EXIT}"
   case "${trap_sig}" in
   0|EXIT) msg "exiting";;
   *) msg "received SIG${trap_sig}";;
   esac

   trap - ${trap_sigs}

   cleanup "${trap_sig}"

   case "${trap_sig}" in
   ""|0|EXIT) ;;
   *) kill -"${trap_sig}" "$$";;
   esac
}

for trap_sig in ${trap_sigs}
do
   trap "trap_func ${trap_sig}" ${trap_sig}
done

# Kill the given PID, ensuring that it is a child of this script ($$).
kill_child() {
   local pid
   pid="${1}"
   case $(($(ps -oppid= -p"${pid}" || :) + 0)) in
   $$) ;;
   *) return 1;;
   esac
   msg "killing pid ${pid}"
   kill "${pid}"
}

# Kill nodes that are direct child of this script (pid $$),
# i.e. run directly from main loop.
kill_node() {
   local pids pid delay

   msg "finding node processes that are our children"
   pids=$(
      ps axcwwo "pid=,ppid=,command=" |
      awk -v me=$$ '$2 == me && $3 == "harmony" { print $1; }'
   )
   msg "found node processes: ${pids:-"<none>"}"
   for pid in ${pids}
   do
      delay=0
      while kill_child ${pid}
      do
         sleep ${delay}
         delay=1
      done
      msg "pid ${pid} no longer running"
   done
}

{
   while :
   do
      msg "re-downloading binaries in 5m"
      sleep 300
      if any_new_binaries staging
      then
         msg "binaries did not change"
         continue
      fi
      while ! download_binaries staging
      do
         msg "staging download failed; retrying in 30s"
         sleep 30
      done
      if diff staging/harmony-checksums.txt harmony-checksums.txt
      then
         msg "binaries did not change"
         continue
      fi
      msg "binaries changed; moving from staging into main"
      (cd staging; exec mv harmony-checksums.txt "${BIN[@]}" ..) || continue
      msg "binaries updated, killing node to restart"
      kill_node
   done
} > harmony-update.out 2>&1 &
check_update_pid=$!

if [ -z "${blspass}" ]; then
   unset -v passphrase
   read -rsp "Enter passphrase for the BLS key file ${BLSKEYFILE}: " passphrase
   echo
elif [ ! -f "${blspass}" ]; then
   err 10 "can't find the ${blspass} file"
fi

while :
do
   msg "############### Running Harmony Process ###############"
   args=(
      -bootnodes "${BN_MA}"
      -ip "${PUB_IP}"
      -port "${NODE_PORT}"
      -is_genesis
      -blskey_file "${BLSKEYFILE}"
      -network_type="${network_type}"
      -dns_zone="${dns_zone}"
   )
   case "${metrics}" in
   true)
      args+=(
         -metrics "${metrics}"
         -pushgateway_ip "${PUSHGATEWAY_IP}"
         -pushgateway_port "${PUSHGATEWAY_PORT}"
      )
      ;;
   esac
   case "$OS" in
   Darwin) ld_path_var=DYLD_FALLBACK_LIBRARY_PATH;;
   *) ld_path_var=LD_LIBRARY_PATH;;
   esac
   run() {
      env "${ld_path_var}=$(pwd)" ./harmony "${args[@]}" "${@}"
   }
   case "${blspass:+set}" in
   "") echo -n "${passphrase}" | run -blspass stdin;;
   *) run -blspass file:${blspass};;
   esac || msg "node process finished with status $?"
   ${loop} || break
   msg "restarting in 10s..."
   sleep 10
done
