#!/usr/bin/env bash
./test/kill_node.sh
rm -rf tmp_log*
rm *.rlp
rm -rf .dht*
./test/deploy.sh -D 60000 ./test/configs/local-resharding.txt
