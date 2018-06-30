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
source ~/.bash_profile
echo "Inside deploy linux"
echo $GOPATH
echo "Inside deploy linux line 2"

config=$1
while read ip port mode; do 
	#echo $ip $port $mode $config
  go run ./benchmark.go -ip $ip -port $port -config_file $config&
done < $config
go run ./client/txgen/main.go -config_file $config
