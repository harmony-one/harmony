#!/bin/bash

# list of process name to be killed
targetProcess=( "harmony" "bootnode" "soldier" "commander" "profiler" )

NAME="$(uname -s)"

for pname in "${targetProcess[@]}"
do
  if [ $NAME == "Darwin" ]; then
    pkill -9 "^(${pname})$"
  else
    pkill -9 -e "^(${pname})$"
  fi
done

rm -rf db-127.0.0.1-*