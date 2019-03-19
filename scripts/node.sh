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
   PUB_IP=$(dig @resolver1.opendns.com ANY myip.opendns.com +short)
   if valid_ip $PUB_IP; then
      echo MYIP = $PUB_IP
   else
      echo NO valid public IP found: $PUB_IP
      exit 1
   fi
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

   echo "* soft     nproc          65535" | sudo tee -a /etc/security/limits.conf
   echo "* hard     nproc          65535" | sudo tee -a /etc/security/limits.conf
   echo "* soft     nofile         65535" | sudo tee -a /etc/security/limits.conf
   echo "* hard     nofile         65535" | sudo tee -a /etc/security/limits.conf
   echo "root soft     nproc          65535" | sudo tee -a /etc/security/limits.conf
   echo "root hard     nproc          65535" | sudo tee -a /etc/security/limits.conf
   echo "root soft     nofile         65535" | sudo tee -a /etc/security/limits.conf
   echo "root hard     nofile         65535" | sudo tee -a /etc/security/limits.conf
   echo "session required pam_limits.so" | sudo tee -a /etc/pam.d/common-session
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
   echo Please use \"sudo ./node.sh\"
   exit 1
fi

killnode

mkdir -p latest
BUCKET=pub.harmony.one
OS=$(uname -s)
REL=banjo

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/$REL/darwin-x86_64/
   BIN=( harmony libbls384.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
   export DYLD_FALLBACK_LIBRARY_PATH=$(pwd)
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/$REL/linux-x86_64/
   BIN=( harmony libbls384.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
   export LD_LIBRARY_PATH=$(pwd)
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
BN_MA=/ip4/100.26.90.187/tcp/9876/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9,/ip4/54.213.43.194/tcp/9876/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX

if [ "$OS" == "Linux" ]; then
# Run Harmony Node
   LD_LIBRARY_PATH=$(pwd) nohup ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_beacon > harmony-${PUB_IP}.log 2>&1 &
else
   DYLD_FALLBACK_LIBRARY_PATH=$(pwd) ./harmony -bootnodes $BN_MA -ip $PUB_IP -port $NODE_PORT -is_beacon > harmony-${PUB_IP}.log 2>&1 &
fi

echo "############### Running Harmony Process ###############"
find_harmony_process
echo
echo

echo Please run the following command to inspect the log
echo "tail -f harmony-${PUB_IP}.log"

echo
echo You may use \"sudo kill harmony\" to terminate running harmony node program.

trap killnode SIGINT SIGTERM
