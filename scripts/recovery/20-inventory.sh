#!/usr/bin/env bash
# Phase A (optional): minimal namespace accounting for retention/size
# decisions (plan WS2).
#
#   RECOVERY_REPORT_DIR=/reports COPY=/data/aug8-a/harmony_db_0 \
#   scripts/recovery/20-inventory.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require COPY "${COPY:-}"

recovery_run inventory-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COPY}" --read-only \
  --output "${RECOVERY_REPORT_DIR}/inventory.json"

echo "inventory: ${RECOVERY_REPORT_DIR}/inventory.json"
