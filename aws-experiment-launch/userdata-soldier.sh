#!/bin/bash
yum install ruby -y
cd /home/ec2-user/
curl http://unique-bucket-bin.s3.amazonaws.com/txgen -o txgen
curl http://unique-bucket-bin.s3.amazonaws.com/soldier -o soldier
curl http://unique-bucket-bin.s3.amazonaws.com/benchmark -o benchmark
chmod +x ./soldier
chmod +x ./txgen
chmod +x ./benchmark

# Get My IP
ip=`curl http://169.254.169.254/latest/meta-data/public-ipv4`

NODE_PORT=9000
SOLDIER_PORT=1$NODE_PORT
# Kill existing soldier/node
fuser -k -n tcp $SOLDIER_PORT
fuser -k -n tcp $NODE_PORT

# Run soldier
./soldier -ip $ip -port $NODE_PORT > soldier_log 2>&1 &