#!/bin/bash

# list of process name to be killed
targetProcess=( "harmony" "bootnode" "soldier" "commander" "profiler" )

for pname in "${targetProcess[@]}"
do
  sudo pkill -9 -e "^(${pname})$"
done

rm -rf db-127.0.0.1-*