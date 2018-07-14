#!/bin/bash
curl http://54.186.246.9:8080/soldier -o soldier
chmod +x ./soldier
curl http://54.186.246.9:8080/benchmark -o benchmark
chmod +x ./benchmark
curl http://54.186.246.9:8080/txgen -o txgen
chmod +x ./txgen

# Get My IP
wget http://169.254.169.254/latest/meta-data/public-ipv4
ip=$(head -n 1 public-ipv4)
# Run soldier
node_port=9000
./soldier -ip $ip -port $node_port > soldier_log 2>&1 &