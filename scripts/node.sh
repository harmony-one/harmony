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

function add_env
{
   filename=$1
   shift
   grep -qxF "$@" $filename || echo "$@" >> $filename
}

function setup_env
{
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
if [[ $EUID -ne 0 ]]; then
   msg "this script must be run as root"
   msg please use \"sudo $0\"
   exit 1
fi

usage() {
   msg "$@"
   cat <<- ENDEND
	usage: ${progname} [-b] account_address
	-c              back up database/logs and start clean
	ENDEND
   exit 64  # EX_USAGE
}

unset start_clean loop
start_clean=false
loop=true

unset OPTIND OPTARG opt
OPTIND=1
while getopts :c1 opt
do
   case "${opt}" in
   '?') usage "unrecognized option -${OPTARG}";;
   ':') usage "missing argument for -${OPTARG}";;
   c) start_clean=true;;
   1) loop=false;;
   *) err 70 "unhandled option -${OPTARG}";;  # EX_SOFTWARE
   esac
done
shift $((${OPTIND} - 1))

case $# in
0)
   usage "Please provide account address." \
      "For foundational nodes, please follow the instructions in Discord #foundational-nodes channel" \
      "to generate and register your account address with <genesis at harmony dot one>."
   ;;
esac

IDX="${1}"
shift 1

case $# in
[1-9]*)
   usage "extra arguments at the end ($*)"
   ;;
esac

BUCKET=pub.harmony.one
OS=$(uname -s)
REL=drum

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/darwin-x86_64/$REL/
   BIN=( harmony libbls384_256.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/linux-x86_64/$REL/
   BIN=( harmony libbls384_256.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
fi

# clean up old files
for bin in "${BIN[@]}"; do
   rm -f ${bin}
done

download_binaries() {
   local outdir
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

if [ "$OS" == "Linux" ]; then
   setup_env
# Kill existing soldier/node
   fuser -k -n tcp $NODE_PORT
fi

# find my public ip address
myip

# public boot node multiaddress
BN_MA=/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv,/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9

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

while :
do
   msg "############### Running Harmony Process ###############"
   if [ "$OS" == "Linux" ]; then
   # Run Harmony Node
      LD_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -accounts $IDX
   else
      DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -accounts $IDX
   fi || msg "node process finished with status $?"
   ${loop} || break
   msg "restarting in 10s..."
   sleep 10
done
