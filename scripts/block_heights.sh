#!/bin/bash
#this file was written for the exclusive pleasure of Mr. John Whitton
#TODO undo hardcoding of files

function usage
{
   ME=$(basename $0)
   cat<<EOT
Usage: $ME [OPTIONS]

This script gets the block heights of each IP in a given network.

[OPTIONS]
   -a               height
   -n               clear files
   -p  string       profile (e.g. beta, main)
   -s               sync logs


[EXAMPLES]

   $ME -p beta -n -a

EOT
   exit 0
}

verbose=false
profile='b'

sync=false

while getopts 'hnap:' flag; do
  case "${flag}" in
    h) usage ;;
    a) blockHeight=true ;;
    n) clear=true ;;
    p) profile=${OPTARGS} ;;
    s) sync=true ;;
    *) usage ;;
  esac
done

#create new files
echo creating new files...
if [ $clear ]
then
  > heights.txt
  > shard0.txt
  > shard1.txt
  > shard2.txt
  > shard3.txt
  > sorted.txt
fi

if [ $sync ]
then
    case "$profile" in 
        beta)
            WHOAMI=BETA 
            HMY_PROFILE=beta
            ;;
        main)
            WHOAMI=HARMONY 
            HMY_PROFILE=s3 
            ;;
    esac

    echo syncing logs...
    ./home/ec2-user/experiment-deploy/pipeline/sync_logs.sh -q
fi


printf "getting heights...\n"
printf "ShardID Role BlockHeight IP\n\n" >> heights.txt
tac /home/ec2-user/experiment-deploy/pipeline/logs/$HMY_PROFILE/distribution_config.txt | while read line
do 
    ip="$(echo $line | awk '{print$1}')"
    val=-1
    val="$(curl -X POST \
    http://${ip}:9500 \
    -H 'Accept: */*' \
    -H 'Accept-Encoding: gzip, deflate' \
    -H 'Cache-Control: no-cache' \
    -H 'Connection: keep-alive' \
    -H 'Content-Length: 63' \
    -H 'Content-Type: application/json' \
    -H 'Host: 34.238.42.51:9500' \
    -H 'Postman-Token: 0ee86467-cc6e-4834-9336-2c3cc54dfad8,380962ea-8dc5-4206-9321-b1fbbfc2c80c' \
    -H 'User-Agent: PostmanRuntime/7.16.3' \
    -H 'cache-control: no-cache' \
    -d '{"jsonrpc":"2.0","method":"hmy_blockNumber","params":[],"id":1}' \
    | awk -F '[\"]' '{print $10}')"
    if [ $val == "-1" ]
    then
        printf "$ip ERROR"
    else
        echo $line | awk '{printf "%s %s ", $4, $3}' >> heights.txt
        printf $((16#${val:2}))" " >> heights.txt
        echo $line | awk '{print$1}' >> heights.txt
    fi
done
sort -k3 -r -n heights.txt >> sorted.txt


for i in 0 1 2 3
do 
    grep -E "$i val|$i exp|$i lead" sorted.txt >> shard$i.txt
done

echo

printf "Results Printed to heights.txt and shard*.txt\n"
