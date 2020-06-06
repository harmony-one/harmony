./test/kill_node.sh
rm -rf tmp_log*
rm *.rlp
./test/deploy.sh -D 60000 ./test/configs/local-resharding.txt
