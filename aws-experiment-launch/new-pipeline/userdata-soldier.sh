#!/bin/bash
yum install ruby -y
cd /home/ec2-user/
curl http://unique-bucket-bin.s3.amazonaws.com/txgen -o txgen
curl http://unique-bucket-bin.s3.amazonaws.com/soldier -o soldier
curl http://unique-bucket-bin.s3.amazonaws.com/commander -o commander
curl http://unique-bucket-bin.s3.amazonaws.com/benchmark -o benchmark
chmod +x ./soldier
chmod +x ./txgen
chmod +x ./benchmark
chmod +x ./commander

# Get My IP
ip=`curl http://169.254.169.254/latest/meta-data/public-ipv4`

SOLDIER_PORT=9000
# Kill existing soldier
fuser -k -n tcp $SOLDIER_PORT

# Run soldier
./soldier -ip $ip -port $SOLDIER_PORT > soldier_log 2>&1 &