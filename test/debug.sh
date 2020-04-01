./test/kill_node.sh
rm -rf tmp_log*
./test/deploy.sh -D 60000 ./test/configs/local-resharding.txt
