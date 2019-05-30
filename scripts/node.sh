#!/bin/bash

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

if [ -z "$1" ]; then
   echo "Usage: $0 account_index_number"
   echo
   echo "Please provide account index."
   echo "For foundational nodes, please follow the instructions in discord #foundational-nodes channel"
   echo "to get your account index number."
   echo
   exit 1
fi

IDX=$1

killnode

mkdir -p latest
BUCKET=pub.harmony.one
OS=$(uname -s)
REL=drum

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/darwin-x86_64/$REL/
   BIN=( harmony libbls384.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/linux-x86_64/$REL/
   BIN=( harmony libbls384.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
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

echo "############### Running Harmony Process ###############"
if [ "$OS" == "Linux" ]; then
# Run Harmony Node
   LD_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -account_index $IDX
else
   DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_genesis -is_archival -account_index $IDX
fi

find_harmony_process
echo
echo

# echo Please run the following command to inspect the log
# echo "tail -f harmony-${PUB_IP}.log"

echo
echo You may use \"sudo pkill harmony\" to terminate running harmony node program.

trap killnode SIGINT SIGTERM
