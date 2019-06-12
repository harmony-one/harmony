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

function killnode() {
   local port=$1

   if [ -n "port" ]; then
      pid=$(/bin/ps -fu $USER | grep "harmony" | grep "$port" | awk '{print $2}')
      echo "killing node with port: $port"
      $DRYRUN kill -9 $pid 2> /dev/null
      echo "node with port: $port is killed"
   fi
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
      echo MYIP = $PUB_IP
   else
      echo NO valid public IP found: $PUB_IP
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

function find_harmony_process
{
   unset -v pidfile pid
   pidfile="harmony-${PUB_IP}.pid"
   pid=$!
   echo "${pid}" > "${pidfile}"
   ps -f -p "${pid}"
}

######## main #########
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root"
   echo Please use \"sudo $0\"
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

unset start_clean
start_clean=false

unset OPTIND OPTARG opt
OPTIND=1
while getopts :c opt
do
   case "${opt}" in
   '?') usage "unrecognized option -${OPTARG}";;
   ':') usage "missing argument for -${OPTARG}";;
   c) start_clean=true;;
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

killnode

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

# download all the binaries
for bin in "${BIN[@]}"; do
   curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o ${bin}
done
chmod +x harmony

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

echo "############### Running Harmony Process ###############"
if [ "$OS" == "Linux" ]; then
# Run Harmony Node
   LD_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -accounts $IDX
else
   DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -accounts $IDX
fi

find_harmony_process
echo
echo

# echo Please run the following command to inspect the log
# echo "tail -f harmony-${PUB_IP}.log"

echo
echo You may use \"sudo pkill harmony\" to terminate running harmony node program.

trap killnode SIGINT SIGTERM
