#!/bin/bash

for pid in `/bin/ps -fu $USER| grep "harmony\|txgen\|soldier\|commander\|profiler\|beacon\|bootnode" | grep -v "grep" | grep -v "vi" | awk '{print $2}'`;
do
    echo 'Killed process: '$pid
    kill -9 $pid
done

rm -rf db-127.0.0.1-*
