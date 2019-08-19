./test/kill_node.sh
rm -rf tmp_log*
./test/deploy.sh -D 600 ./test/configs/local-resharding.txt
