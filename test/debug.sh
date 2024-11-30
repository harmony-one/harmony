#!/usr/bin/env bash

unset -v configfile
configfile=$1
Localnet_Blocks_Per_Epoch=$2
Localnet_Blocks_Per_Epoch_V2=$3

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
# Check if Localnet_Blocks_Per_Epoch is not empty
if [[ -n "$Localnet_Blocks_Per_Epoch" ]]; then
    echo "[INFO] - blocks per epoch is set to $Localnet_Blocks_Per_Epoch"
    BLOCK_PER_EPOCH_OPT=(-K "${Localnet_Blocks_Per_Epoch}")
fi
# Check if Localnet_Blocks_Per_Epoch_V2 is not empty
if [[ -n "$Localnet_Blocks_Per_Epoch_V2" ]]; then
    echo "[INFO] - blocks per epoch v2 is set to $Localnet_Blocks_Per_Epoch_V2"
    BLOCK_PER_EPOCH_V2_OPT=(-E "${Localnet_Blocks_Per_Epoch_V2}")
fi

if [ "${VERBOSE}" = "true" ]; then
    echo "[WARN] - running with verbose logs"
    ./test/deploy.sh -v -B -D 600000 "${BLOCK_PER_EPOCH_OPT[@]}" "${BLOCK_PER_EPOCH_V2_OPT[@]}" "${SYNC_OPT[@]}" "${configfile}"
else
    ./test/deploy.sh -B -D 600000 "${BLOCK_PER_EPOCH_OPT[@]}" "${BLOCK_PER_EPOCH_V2_OPT[@]}" "${SYNC_OPT[@]}" "${configfile}"
fi
