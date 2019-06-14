#!/bin/bash

function usage
{
   cat<<EOT
Usage: $0 [option] command

Option:
   -h          print this help

Monitor Help:

Actions:
    1. status        - Generates a status report of your node
EOT
	exit 0
}


while getopts "h" opt; do
	case $opt in
		h) usage ;;
		*) usage ;;
	esac
done

cd $HOME
#check if you're in snych
heightStatus=$(grep otherHeight ./latest/validator-54.221.12.96-9000.log |  egrep  -o "myHeight(.*)([0-9]+)," | tail -n 1)

# Which Shard
my_shard=$(egrep -o "shard\/[0-9]+" ./latest/validator*.log | tail -n 1)

# Which IP
ip=$(curl http://169.254.169.254/latest/meta-data/public-ipv4)

#check if you're in snych

#Reward
./wallet.sh balances

echo "Your Sync Status: "$heightStatus
echo "Your Shard: " $my_shard
echo "Your IP: "$ip
echo "You Reward: "
./wallet.sh balances



# display the first block you started receiving

# show the percentage of earned / recieved

#Is your account registered
# ./wallet.sh format --address one1xhffyq90exjvmsz3vcykqkdqggrcedf7zdcvt8
# account address in Bech32: one1xhffyq90exjvmsz3vcykqkdqggrcedf7zdcvt8
# account address in Base16 (deprecated): 0x35D29200aFC9A4cDC05166096059a042078CB53e

# https://raw.githubusercontent.com/harmony-one/harmony/master/internal/genesis/foundational.go