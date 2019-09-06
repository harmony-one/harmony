./test/kill_node.sh
rm -rf tmp_log*
./test/deploy.sh -D 60000000 ./test/configs/local-resharding.txt &
sleep 100s
rm api_test_results.log
./scripts/api_test.sh -l -v > api_test_results.log
cat api_test_results.log
