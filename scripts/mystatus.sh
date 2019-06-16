#!/usr/bin/env bash

usage () {
   cat << EOT
Usage: $0 [option] command

Option:
    -h      print this help

Monitor Help:

Actions:
    1. status       - Generates a status report of your node
EOT
}

valid_ip () {
# https://www.linuxjournal.com/content/validating-ip-address-bash-script
    local  ip=$1
    local  stat=1

    if [[ $ip =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        OIFS=$IFS
        IFS='.'
        ip=($ip)
        IFS=$OIFS
        [[ ${ip[0]} -le 255 && ${ip[1]} -le 255 \
            && ${ip[2]} -le 255 && ${ip[3]} -le 255 ]]
        stat=$?
    fi
    return $stat
}

status_report () {
    # Block heights
    heightStatus=$(tac latest/validator*.log | grep -Eom1 '"myHeight":[0-9]+' | cut -d: -f2)
    lengthOfChain=$(tac latest/validator*.log | grep -Eom1 '"otherHeight":[0-9]+' | cut -d: -f2)

    # Shard number
    my_shard=$(grep -Eom1 "shardID\"\:[0-9]+" latest/validator*.log | cut -d: -f2)

    # Public IP
    ip=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)

    # Check validity of IP
    if ! valid_ip $ip; then
        echo "NO valid public IP found: $ip"
        exit 2
    fi

    # Number of bingos
    bingos=$(grep -c "BINGO" ./latest/validator*log)

    #echo "Your Node Version : "
    LD_LIBRARY_PATH=$(pwd) ./harmony -version
    echo "Current Length of Chain:" $lengthOfChain
    echo "Your Sync Status:" $heightStatus
    echo "Your Shard:" $my_shard
    echo "Your IP:" $ip
    echo "Total Blocks Received  After Syncing:" $bingos
    echo "Your Rewards:"
    ./wallet.sh balances
}

#####Main#####

while getopts "h" opt; do
    case $opt in
        h|*)
            usage
            exit 1
            ;;
    esac
done
shift $(($OPTIND-1))

[ $# -eq 0 -o $# -gt 1 ] && usage && exit 1

case "$1" in
    status)
        status_report
        ;;
    *)
        usage;
        exit 1
        ;;
esac