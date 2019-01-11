#!/bin/bash

function usage
{
   ME=$(basename $0)
   cat<<EOT
Usage: $ME list_of_leaders list_of_validators

This script calculate the number of consensus and TPS based on leader log and validator log files.

list_of_leaders contains a list of leader log files
list_of_validators contains a list of validator log files

EOT
   exit 0
}

if [ "$#" -ne 2 ]; then
   usage
fi

LEADERS=$1
VALIDATORS=$2

FILES=( $(cat $LEADERS) )

if [ -n "$VALIDATORS" ]; then
   NUM_VALIDATORS=$(wc -l $VALIDATORS | awk ' { print $1 } ')
fi

NUM_SHARDS=${#FILES[@]}
SUM=0
NUM_CONSENSUS=0

declare -A TPS

for f in "${FILES[@]}"; do
   leader=$( echo $(basename $f) | cut -f 2 -d\- )
   num_consensus=$(grep HOORAY $f | wc -l)
   if [ $num_consensus -gt 0 ]; then
      avg_tps=$(grep TPS $f | cut -f 2 -d: | cut -f 1 -d , | awk '{s+=$1} END {print s/NR}')
      printf -v avg_tps_int %.0f $avg_tps
   else
      avg_tps=0
   fi
   TPS[$leader]="$num_consensus, $avg_tps"
   NUM_CONSENSUS=$(expr $NUM_CONSENSUS + $num_consensus )
   SUM=$( expr $SUM + $avg_tps_int )
done

echo $NUM_SHARDS shards, $NUM_CONSENSUS consensus, $SUM total TPS, $NUM_VALIDATORS nodes
for t in "${!TPS[@]}"; do
   echo $t, ${TPS[$t]}
done
