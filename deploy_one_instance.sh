#!/bin/bash -x
##The commented suffix is for linux
##Reference: https://github.com/Zilliqa/Zilliqa/blob/master/tests/Node/test_node_simple.sh
sudo sysctl net.core.somaxconn=1024
sudo sysctl net.core.netdev_max_backlog=65536;
sudo sysctl net.ipv4.tcp_tw_reuse=1;
sudo sysctl -w net.ipv4.tcp_rmem='65536 873800 1534217728';
sudo sysctl -w net.ipv4.tcp_wmem='65536 873800 1534217728';
sudo sysctl -w net.ipv4.tcp_mem='65536 873800 1534217728';

./kill_node.sh
MyHOME=/home/ec2-user
source ~/.bash_profile
export GOROOT=/usr/lib/golang
export GOPATH=$MyHOME/projects
export PATH=$PATH:$GOROOT/bin
wget http://169.254.169.254/latest/meta-data/public-ipv4  #Calling for public IPv4
current_ip=$(head -n 1 public-ipv4)
echo "Current IP is >>>"
echo $current_ip
echo ">>>>"
python aws-scripts/preprocess_peerlist.py 
FILE='isTransaction.txt'
config=$1

t=`date +"%Y%m%d-%H%M%S"`
log_folder="logs/log-$t"

if [ ! -d $log_folder ] 
then
    mkdir -p $log_folder
fi

if [ -f $FILE ]; then
    go run ./client/txgen/main.go -config_file $config -log_folder $log_folder&
else
    go run ./benchmark_main.go -ip $current_ip -config_file $config -log_folder $log_folder&
fi