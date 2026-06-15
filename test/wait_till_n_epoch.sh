#! /bin/bash
set -euo pipefail

TARGET_EPOCH=${1:?target epoch is required}
NODE_URL=${NODE_URL:-http://localhost:9500}
TIMEOUT_SECONDS=${TIMEOUT_SECONDS:-180}
INTERVAL_SECONDS=${INTERVAL_SECONDS:-1}

if [[ "${NODE_URL}" != http://* && "${NODE_URL}" != https://* ]]; then
	NODE_URL="http://${NODE_URL}"
fi

deadline=$((SECONDS + TIMEOUT_SECONDS))
while ((SECONDS < deadline)); do
	response=$(curl -fsS -X POST -H "Content-Type: application/json" \
		--data '{"jsonrpc":"2.0","method":"hmyv2_getLatestChainHeaders","params":[],"id":1}' \
		"${NODE_URL}" 2>/dev/null || true)
	epoch=$(echo "${response}" | jq -r '.result["beacon-chain-header"].epoch // empty' 2>/dev/null || true)
	if [[ -n "${epoch}" && "${epoch}" != "null" ]] && ((epoch >= TARGET_EPOCH)); then
		echo "we are on the epoch ${epoch}, let's proceed"
		exit 0
	fi
	echo "Not yet on epoch ${TARGET_EPOCH}; current=${epoch:-unknown}; waiting ${INTERVAL_SECONDS}s"
	sleep "${INTERVAL_SECONDS}"
done

echo "Timed out waiting ${TIMEOUT_SECONDS}s for epoch ${TARGET_EPOCH} at ${NODE_URL}" >&2
exit 1
