#!/bin/bash -x
REGION=$(curl 169.254.169.254/latest/meta-data/placement/availability-zone/ | sed 's/[a-z]$//')
#yum update -y  #This breaking codedeploy right now
yum install ruby wget -y
cd /home/ec2-user
touch yum-not-updated.txt
wget https://aws-codedeploy-$REGION.s3.amazonaws.com/latest/install
chmod +x ./install
./install auto
mkdir projects
mkdir projects/src