#!/usr/bin/env bash

## sudo yum install -q -y jq

usage () {
   cat << EOT
Usage: $0 [option] command

Option:
    -h      print this help

Actions:
    1. health       - Generates a status report of your node
    2. address      - Checks if your wallet is registered in the FN list
    3. all          - Does all above
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

health_report () {
    # Block heights
    lastSynchBlock=$(tac latest/validator*.log | grep -Eoim1 '"OtherHeight":[0-9]+' | cut -d: -f2)
    [ -z $lastSynchBlock ] && lastSynchBlock=Unknown

    chainLength=$(jq -r 'select(.msg == "[TryCatchup] Adding block to chain") | .myBlock' ./latest/v*.log | tail -1 | cut -d: -f2)
    [ -z "$chainLength" ] && chainLength=Unknown

    synchStatus=$(jq -r 'select(.msg == "Node is in sync" or .msg == "Node is out of sync") | .msg' ./latest/v*.log | tail -1 | cut -d: -f2)
    [ -z "$synchStatus" ] && synchStatus=Unknown

    lengthOfChain=$(tac latest/validator*.log | grep -Eoim1 '"OtherHeight":[0-9]+' | cut -d: -f2)
    [ -z $lengthOfChain ] && lengthOfChain=Unknown

    # Shard number
    my_shard=$(grep -Eom1 "shardID\"\:[0-9]+" latest/validator*.log | cut -d: -f2)
    [ -z $my_shard ] && lengthOfChain=Unknown

    # Public IP
    ip=$(dig -4 @resolver1.opendns.com ANY myip.opendns.com +short)

    # Check validity of IP
    if ! valid_ip $ip; then
        echo "NO valid public IP found: $ip"
        exit 2
    fi

    # Number of bingos
    bingos=$(grep -c "BINGO" ./latest/validator*log)
    [ -z bingos ] && bingos=0

    #echo "Your Node Version : "
    echo -e "\n====== HEALTH ======\n"
    LD_LIBRARY_PATH=$(pwd) ./harmony -version
    echo "Current Length of Chain:" $chainLength
    echo "Your Sync Status:" $synchStatus
    echo "Latest Block Synchronized:" $lastSynchBlock
    echo "Your Chain Length:" $chainLength
    echo "Your Shard:" $my_shard
    echo "Your IP:" $ip
    echo "Total Blocks Received After Syncing:" $bingos
    ./wallet.sh balances
}

address_report () {
    filename=$(find .hmy/keystore -maxdepth 1 -type f | head -n1)
    address_field=$(grep -o '"address":"[a-z0-9]*' "${filename}")
    base16=$(cut -d\" -f4 <<< "${address_field}")
    bech32=$(./wallet.sh format --address 0x${base16} | head -n1 | awk -F: '{print $2}' | awk '{$1=$1}1')
    curl -s https://raw.githubusercontent.com/harmony-one/harmony/master/internal/genesis/foundational.go | grep -qom1 "${bech32}"
    echo -e "\n====== ADDRESS ======\n"
    if [ $? -eq 0 ]; then
        echo "SUCCESS: "${bech32}" FOUND in our foundational list!"
    else
        echo "FAILURE: "${bech32}" NOT FOUND in our foundational list, check if you have the right keyfile."
    fi
}

#####Main#####

while getopts "h" opt; do
    case $opt in
        h|*)
            echo "GETOPTS"
            usage
            exit 1
            ;;
    esac
done
shift $(($OPTIND - 1))

[ $# -eq 0 ] && address_report && health_report && exit 1

while :; do
    case "$1" in
        health)
            health_report
            shift
            ;;
        address)
            address_report
            shift
            ;;
        all)
            address_report
            health_report
            shift
            ;;
        '')
            echo "EMPTY"
            exit 0
            ;;
        *)
            echo "**********"
            usage
            exit 1
            ;;
    esac 
done
