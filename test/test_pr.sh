#!/usr/bin/env bash
# This script is meant to be run inside the docker image: harmonyone/test-pr
echo ==============
echo STARTING TESTS
echo ==============
echo
echo "You can \`tail\` these logs to see the current progress..."
if [ "$1" == "travis" ]; then
    echo "Go test logs are saved in \`./pr_log/go_test.log\`"
    echo "Running go tests..."
    cd ${HMY_PATH}/harmony && ./scripts/travis_checker.sh > ${HMY_PATH}/harmony/pr_log/go_test.log
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ GO TEST LOGS ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
    cat ${HMY_PATH}/harmony/pr_log/go_test.log
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
else
    echo "Localnet debug logs saved in: \`./pr_log/localnet.log\`"
    echo "Integration test logs saved in: \`./pr_log/integration_test.log\`"

    # Manually clean all files
    rm -rf ${HMY_PATH}/harmony/db-*
    rm -rf ${HMY_PATH}/harmony/.dht-*
    rm -rf ${HMY_PATH}/harmony/bin/
    rm -rf ${HMY_PATH}/harmony/tmp_log/
    rm -rf ${HMY_PATH}/harmony/pr_log/
    mkdir -p ${HMY_PATH}/harmony/pr_log/

    echo "Running integration tests..."
    echo "Binary Version(s)":
    cd ${HMY_PATH}/harmony && ./test/debug.sh > ${HMY_PATH}/harmony/pr_log/localnet.log || true &
    cd ${HMY_PATH}/harmony-ops/test-automation/api-tests/ && ./localnet_test.sh -w 90 -t 180 -d 30 -i 5 > ${HMY_PATH}/harmony/pr_log/integration_test.log
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ LOCALNET LOGS ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
    cat ${HMY_PATH}/harmony/pr_log/localnet.log
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
    echo 
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~ INTEGRATION TEST LOGS ~~~~~~~~~~~~~~~~~~~~~~~~~~"
    cat ${HMY_PATH}/harmony/pr_log/integration_test.log
    echo "~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~"
    tail -20 ${HMY_PATH}/harmony/pr_log/integration_test.log | python3 /usr/local/bin/check_test_pr.py  # Assumes <= 15 tests in the final report
fi
echo 
echo ==============
echo FINISHED TESTS
echo ==============