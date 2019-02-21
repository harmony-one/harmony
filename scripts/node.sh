#!/bin/bash

mkdir -p latest
BUCKET=pub.harmony.one
OS=$(uname -s)
REL=20190216

if [ "$OS" == "Darwin" ]; then
   FOLDER=release/$REL/darwin-x86_64/
   BIN=( harmony libbls384.dylib libcrypto.1.0.0.dylib libgmp.10.dylib libgmpxx.4.dylib libmcl.dylib )
   export DYLD_FALLBACK_LIBRARY_PATH=.
fi
if [ "$OS" == "Linux" ]; then
   FOLDER=release/$REL/linux-x86_64/
   BIN=( harmony libbls384.so libcrypto.so.10 libgmp.so.10 libgmpxx.so.4 libmcl.so )
   export LD_LIBRARY_PATH=.
fi

# download all the binaries
for bin in "${BIN[@]}"; do
   curl http://${BUCKET}.s3.amazonaws.com/${FOLDER}${bin} -o ${bin}
done
chmod +x harmony

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

IS_AWS=$(curl -s -I http://169.254.169.254/latest/meta-data/instance-type -o /dev/null -w "%{http_code}")
if [ "$IS_AWS" != "200" ]; then
# NOT AWS, Assuming Azure
   PUB_IP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-04-02&format=text")
else
   PUB_IP=$(curl http://169.254.169.254/latest/meta-data/public-ipv4)
fi

NODE_PORT=9000
BC_MA=/ip4/54.183.5.66/tcp/9999/ipfs/QmW4PoKvtkBn1CiBjjERXm3QGGohvo3Bn26vJGSgrvdJc4

# Kill existing soldier/node
fuser -k -n tcp $NODE_PORT

# Run Harmony Node
nohup ./harmony -bc_addr $BC_MA -ip $PUB_IP -port $NODE_PORT > harmony-${PUB_IP}.log 2>&1 &

tail -f harmony-${PUB_IP}.log
