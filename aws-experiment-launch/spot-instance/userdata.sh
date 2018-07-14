#!/bin/bash
commanderIP=`curl https://gist.githubusercontent.com/lzl124631x/c44471f85ced6743a173cc113393dc81/raw/4a6b73493f75363de5a61051c8e339294191ca00/commanderIP`
curl http://$commanderIP:8080/soldier -o soldier
chmod +x ./soldier
curl http://$commanderIP:8080/benchmark -o benchmark
chmod +x ./benchmark
curl http://$commanderIP:8080/txgen -o txgen
chmod +x ./txgen

# Get My IP
wget http://169.254.169.254/latest/meta-data/public-ipv4
ip=$(head -n 1 public-ipv4)

node_port=9000
soldier_port=1$node_port
# Kill existing soldier
fuser -k -n tcp $soldier_port

# Run soldier
./soldier -ip $ip -port $node_port > soldier_log 2>&1 &