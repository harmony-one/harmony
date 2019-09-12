#!/bin/bash

function usage
{
   ME=$(basename $0)
   cat<<EOT
Usage: $ME [OPTIONS]

This script gets the top traders on the given network for the given block range.

[OPTIONS]
   -h                print this help message
   -p                specify network(e.g. pangaea, betanet, mainet), DO NOT USE WITH -i
   -v                verbose output
   -s                starting block index, DO NOT USE WITH -t
   -e                ending block index, DO NOT USE WITH -t
   -d                number of shards, DO NOT USE WITH -i
   -i                IP address of explorer, can accept multiple, DO NOT USE WITH -d -p
   -t                startTime according to Epoch Timestamp (https://www.epochconverter.com/#code) in milliseconds rounded to second
   -u                endTime according to Epoch Timestamp (https://www.epochconverter.com/#code) in milliseconds rounded to second
   -d                download block information only
   -a                analyze downloaded block information
   -n                clear block information



[EXAMPLES]

   $ME -p b -s 200 -e 300 -d 0
   $ME -i 52.37.59.120 -i 34.240.4.94 -i 52.91.158.242 -i 52.193.75.216 -t 1568012400000 -u 1568314800000 -s 600000 -e 790000


EOT
   exit 0
}

#keep trying to find a matching time stamp by incrementing every second, assumes that the given times are a close match
function processRaw
{
  printf "processing blocks...\n"
  local sT=$startTime
  local eT=$endTime
   printf "Start Time: "$sT"\n"
  while ! grep $sT raw.txt ;
  do 
    sT=$(($sT + 1000))
    printf .
  done
  printf "End Time: "$eT"\n"
  while ! grep $eT raw.txt ;
  do 
   eT=$(($eT - 1000))
   printf .
  done
  #process the file to remove transactions from outside the time range, then format to only have the sender's address
  cat raw.txt | sed -n '/'$sT'/,$p' | sed '1,/'$eT'/!d' | grep  \"from\" | awk -F "\"" '{print $4}' >> allSent.txt
  cat raw.txt | sed -n '/'$sT'/,$p' | sed '1,/'$eT'/!d' | grep  -A 2 \"from\" | grep -v - | grep -v to | sed '$!N;s/\n/ /' | awk -F"[ \",]" '{print $13" "$27}' >> valSent.txt
  cat raw.txt | sed -n '/'$sT'/,$p' | sed '1,/'$eT'/!d'| grep -B 4 value | grep -v timestamp | grep -v from | grep -v to | grep -v - | sed '$!N;s/\n/ /' | awk -F"[ \",]" '{print $13" "$27}' >> allTxs.txt
}

#get a sum of the volume sent for a single address
function processAccountSentVolume
{
  cat valSent.txt | grep ${address} | awk '{print $2}' > temp.txt
  local sum=0
  > temp1.txt
  cat temp.txt | while read line
  do
    local val="000000000000000000""$(printf "%.0f" ${line})"
    local last="$(echo $val | tail -c 18 | rev | sed 's/^0*//' | rev)"
    local head="$(echo $val | head -c $((${#val}-18)) | sed 's/^0*//')"
    if [ ${#head} -eq 0 ]
    then
      local head=0
    fi
    if [ ${#last} -ne 0 ]
    then 
     local val=$head"."$last
    fi 
    echo $val >> temp1.txt
  done
  (printf $address' ' & awk '{ sum += $1 } END { print sum }' temp1.txt) >> volCount.txt 
}

help=''
verbose='false'
profile='b'
clear=false
downloadOnly=false
analyzeOnly=false

while getopts 'hs:e:b:p:i:s:t:vdnau:' flag; do
  case "${flag}" in
    h) usage ;;
    s) startBlock=${OPTARG} ;;
    t) startTime=${OPTARG} ;;
    u) endTime=${OPTARG} ;;
    e) endBlock=${OPTARG} ;;
    b) ((shards=${OPTARG}-1)) ;;
    p) profile=${OPTARG} ;;
    v) verbose='true' ;;
    i) ips+=("$OPTARG");;
    n) clear=true ;;
    d) downloadOnly=true ;;
    a) analyzeOnly=true ;;
    *) usage ;;
  esac
done

#create new files
if $clear
then
  printf "clearing files...\n"
  > raw.txt
  > allSent.txt 
  > count.txt
  > ranking.txt
  > temp.txt
  > allTxs.txt 
  > vals.txt
  > total.txt
  > valSent.txt
  > volCount.txt
  > volRanking.txt
  > tempVol.txt
fi

#calculate starting time and ending time
if [ -n "$startTime" ] && [ -z "$endTime" ]; then
  endTime=$(($startTime + 86400000))
fi

if !($analyzeOnly)
then
  printf "downloading block information...\n"
  #collects transactions through grepping block info, handles either ip or number of shards inputs
  if [ -n "$ips" ]; then
    for i in "${ips[@]}"; do
      printf 'http://'${i}':5000/blocks?from='${startBlock}'&to='${endBlock}'&offset='$(($endBlock-$startBlock+1))'\n'
      curl -X GET 'http://'${i}':5000/blocks?from='${startBlock}'&to='${endBlock}'&offset='$(($endBlock-$startBlock+1)) | jq . > raw.txt
      processRaw 
    done
  else
    for s in 0 $shards
    do
      curl -X GET   'http://e'${s}'.'${profile}'.hmny.io:5000/blocks?from='${startBlock}'&to='${endBlock}'&offset='$(($endBlock-$startBlock+1)) | jq . > raw.txt
      processRaw
    done  
  fi
  if $downloadOnly
  then
    exit 1
  fi
fi

#count number of transactions for each address and sort them, convert to Bech32 address 
printf processing senders 
while [ -s allSent.txt ]; do
printf .
  address="$(head -n 1 allSent.txt)"
  oneAddress=$address #"$(./go/src/github.com/harmony-one/harmony/bin/wallet format --address $address | grep Bech32 | awk '{print $5}')"
  (printf $oneAddress' ' & (grep ${address} allSent.txt | wc -l)) >> count.txt
  grep -v ${address} allSent.txt > temp.txt
  mv temp.txt allSent.txt
done
printf "\n"

#delete duplicate transaction recordings due to crossshard txs, modify value from scientific to decimal and store in vals.txt
printf processing values
while [ -s allTxs.txt ]; do  
  printf .
  local txHash="$(head -n 1 allTxs.txt | awk '{print $1}')"
  local val="000000000000000000""$(printf "%.0f" "$(head -n 1 allTxs.txt | awk '{print $2}')")"
  local last="$(echo $val | tail -c 18 | rev | sed 's/^0*//' | rev)"
  local head="$(echo $val | head -c $((${#val}-18)) | sed 's/^0*//')"
  if [ ${#head} -eq 0 ]
  then
    local head=0
  fi
  if [ ${#last} -ne 0 ]
  then 
    local val=$head"."$last
  fi 
  echo $val >> vals.txt
  grep -v ${txHash} allTxs.txt > temp.txt
  mv temp.txt allTxs.txt
done
awk '{ sum += $1 } END { print sum }' vals.txt > total.txt
printf "\n"

#count volume sent for each account
printf processing volume
while [ -s valSent.txt ]; do  
  printf .
  address="$(head -n 1 valSent.txt | awk '{print $1}')"
  processAccountSentVolume
  grep -v ${address} valSent.txt > temp.txt
  mv temp.txt valSent.txt
done

printf "\n"
printf "\n"

echo "Address                                    Number of Transactions" > ranking.txt
echo "Address                                    Amount of ONEs Sent" > volRanking.txt
sort -k2 -r -n count.txt >> ranking.txt
sort -k2 -r -n volCount.txt >> volRanking.txt
printf "Total Unique Addresses That Sent Transactions: " >> ranking.txt
wc -l count.txt | awk '{print $1}'>> ranking.txt
printf "Total Number of Transactions: " >> ranking.txt
cat count.txt | awk '{sum+=$2} END{print sum}' >> ranking.txt

echo Complete Sent Transaction Volume Rankings Printed to volRanking.txt
echo Complete Sent Transaction Count Rankings Printed to ranking.txt
printf "\n"
echo Top 3 Senders by Volume && head -n 4 volRanking.txt
printf "\n"
echo Top 3 Senders by Transaction Count && head -n 4 ranking.txt
tail -n 2 ranking.txt
printf "\n"
echo Total ONEs Transferred && cat total.txt



#TODO remove redundant calculation of total ONEs sent