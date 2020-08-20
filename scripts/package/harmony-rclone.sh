#!/bin/bash

if [ $# != 1 ]; then
    echo "usage: $0 datadir"
    exit 1
fi

HARMONY_HOME="$1"

sudo -u harmony mkdir -p ${HARMONY_HOME}/archive
for i in 0 1 2 3; do
  sudo -u harmony rclone sync -vvv hmy:pub.harmony.one/mainnet.min/harmony_db_$i ${HARMONY_HOME}/archive/harmony_db_$i > ${HARMONY_HOME}/archive/archive-$i.log 2>&1
done
