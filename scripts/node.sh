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

killnode

mkdir -p latest
BUCKET=pub.harmony.one
OS=$(uname -s)
REL=20190216

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

# public beacon node multiaddress
BC_MA=/ip4/54.183.5.66/tcp/9999/ipfs/QmdQVypu6NSm7m8bNZj5EJCnjPhXR8QyRmDnDBidxGaHWi

if [ "$OS" == "Linux" ]; then
# Run Harmony Node
   nohup ./harmony -bc_addr $BC_MA -ip $PUB_IP -port $NODE_PORT > harmony-${PUB_IP}.log 2>&1 &
else
   ./harmony -bc_addr $BC_MA -ip $PUB_IP -port $NODE_PORT > harmony-${PUB_IP}.log 2>&1 &
fi

echo Please run the following command to inspect the log
echo "tail -f harmony-${PUB_IP}.log"

trap killnode SIGINT SIGTERM
