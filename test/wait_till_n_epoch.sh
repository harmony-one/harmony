#! /bin/bash

#wait for epoch N
epoch=$(hmy blockchain latest-headers --node="http://localhost:9500" | jq -r '.["result"]["beacon-chain-header"]["epoch"]')
while (( epoch < "${1}" )); do
	echo "Not yet on epoch ${1} .. waiting 30s"
	epoch=$(hmy blockchain latest-headers --node="http://localhost:9500" | jq -r '.["result"]["beacon-chain-header"]["epoch"]')
	sleep 30
done
echo "we are on the epoch ${1}, let's proceed"
