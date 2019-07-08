# to install gsed for MacOS: brew install gnu-sed
./test/kill_node.sh
rm -rf tmp_log*
gsed -i 's/GenesisShardNum = 4/GenesisShardNum = 1/' core/resharding.go
gsed -i 's/GenesisShardSize = 150/GenesisShardSize = 20/' core/resharding.go
gsed -i 's/GenesisShardHarmonyNodes = 112/GenesisShardHarmonyNodes = 20/' core/resharding.go
./test/deploy.sh -D -1 ./test/configs/beaconchain20.txt -network_type testnet
