#!/bin/bash
cd /home/ec2-user
commanderIP= # <- Put the commander IP here.
curl http://$commanderIP:8080/soldier -o soldier
chmod +x ./soldier
curl http://$commanderIP:8080/benchmark -o benchmark
chmod +x ./benchmark
curl http://$commanderIP:8080/txgen -o txgen
chmod +x ./txgen

# Get My IP
ip=`curl http://169.254.169.254/latest/meta-data/public-ipv4`

node_port=9000
soldier_port=1$node_port
# Kill existing soldier
fuser -k -n tcp $soldier_port

# Run soldier
./soldier -ip $ip -port $node_port > soldier_log 2>&1 &