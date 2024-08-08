#!/usr/bin/env bash

./test/kill_node.sh
rm -rf tmp_log* 2> /dev/null
rm *.rlp 2> /dev/null
rm -rf .dht* 2> /dev/null
scripts/go_executable_build.sh -S || exit 1  # dynamic builds are faster for debug iteration...
./test/deploy.sh -B -D 600000 ./test/configs/local-resharding.txt
