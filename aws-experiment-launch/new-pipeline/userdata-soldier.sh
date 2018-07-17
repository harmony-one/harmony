#!/bin/bash
yum install ruby wget -y
cd $HOME
wget http://unique-bucket-bin.s3.amazonaws.com/txgen
wget http://unique-bucket-bin.s3.amazonaws.com/soldier
wget http://unique-bucket-bin.s3.amazonaws.com/benchmark
chmod +x ./soldier
chmod +x ./txgen
chmod +x ./benchmark

# Get My IP
ip=`curl http://169.254.169.254/latest/meta-data/public-ipv4`

SOLDIER_PORT=9000
# Kill existing soldier
fuser -k -n tcp $SOLDIER_PORT

# Run soldier
./soldier -ip $ip -port $SOLDIER_PORT > soldier_log 2>&1 &