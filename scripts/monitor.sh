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

#cd $HOME
#check if you're in snych
heightStatus=$(grep otherHeight ./latest/validator*.log |  egrep  -o "myHeight(.*)([0-9]+)," | tail -n 1)

# Which Shard
my_shard=$(egrep -o "shard\/[0-9]+" ./latest/validator*.log | tail -n 1)

# Which IP
ip=$(curl http://169.254.169.254/latest/meta-data/public-ipv4)

bingos=$(grep -c "BINGO" ./latest/*log)

# balances=$(./wallet.sh balances)

status=$(tac ./latest/* | egrep -m1 'BINGO|HOORAY' | \
    grep ViewID | \
    python -c $'import datetime, sys, json;\nfor x in sys.stdin:\n y = json.loads(x); print "%10s %s %s" % (y.get("ViewID", "*" + str(y.get("myViewID", 0))), datetime.datetime.strftime(datetime.datetime.strptime(y["t"][:-4], "%Y-%m-%dT%H:%M:%S.%f") + datetime.timedelta(hours=-7), "%m/%d %H:%M:%S.%f"), y["ip"])')

#sudo /sbin/ldconfig -v
#nodeVersion=$(LD_LIBRARY_PATH=$(pwd) ./harmony -version)

#check if you're in snych

#Reward

#echo "Your Node Version : "
LD_LIBRARY_PATH=$(pwd) ./harmony -version
echo "Your System Status : "$status
echo "Your Sync Status : "$heightStatus
echo "Your Shard : " $my_shard
echo "Your IP: " $ip
echo "Blocks Received : " $bingos
echo "Your Rewards: "
./wallet.sh balances


# display the first block you started receiving

# show the percentage of earned / recieved

#Is your account registered
# ./wallet.sh format --address one1xhffyq90exjvmsz3vcykqkdqggrcedf7zdcvt8
# account address in Bech32: one1xhffyq90exjvmsz3vcykqkdqggrcedf7zdcvt8
# account address in Base16 (deprecated): 0x35D29200aFC9A4cDC05166096059a042078CB53e

# https://raw.githubusercontent.com/harmony-one/harmony/master/internal/genesis/foundational.go