#!/usr/bin/env bash

./test/kill_node.sh
rm -rf tmp_log* 2> /dev/null
rm *.rlp 2> /dev/null
rm -rf .dht* 2> /dev/null
scripts/go_executable_build.sh -S || exit 1  # dynamic builds are faster for debug iteration...
LEGACY_SYNC=${LEGACY_SYNC:-"false"}
if [ "${LEGACY_SYNC}" == "true" ]; then
    echo "[WARN] - using legacy sync"
    SYNC_OPT=(-L "true")
else
    SYNC_OPT=(-L "false")
fi
if [ "${VERBOSE}" = "true" ]; then
    echo "[WARN] - running with verbose logs"
    ./test/deploy.sh -v -B -D 600000 "${SYNC_OPT[@]}" ./test/configs/local-resharding-with-external.txt
else
    ./test/deploy.sh -B -D 600000 "${SYNC_OPT[@]}" ./test/configs/local-resharding-with-external.txt
fi
