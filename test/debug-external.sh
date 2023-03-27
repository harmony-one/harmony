#!/usr/bin/env bash

./test/kill_node.sh
rm -rf tmp_log*
rm *.rlp
rm -rf .dht*
scripts/go_executable_build.sh -S || exit 1  # dynamic builds are faster for debug iteration...
./test/deploy.sh -B -D 600000 ./test/configs/local-resharding-with-external.txt
