#!/usr/bin/env bash
# Phase A: inspect both Aug 8 copies and emit the two-copy agreement verdict
# (plan WS2). Absolute paths only.
#
#   RECOVERY_REPORT_DIR=/reports \
#   COPY_A=/data/aug8-a/harmony_db_0 COPY_B=/data/aug8-b/harmony_db_0 \
#   ANCHOR=/reports/anchor.json \
#   scripts/recovery/10-inspect.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require COPY_A "${COPY_A:-}"
recovery_require COPY_B "${COPY_B:-}"
recovery_require ANCHOR "${ANCHOR:-}"

A_REPORT="${RECOVERY_REPORT_DIR}/inspect-a.json"
B_REPORT="${RECOVERY_REPORT_DIR}/inspect-b.json"

recovery_run inspect-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COPY_A}" --read-only \
  --full-state-check --full-offchain-check --require-preimages \
  --target-height "${TARGET_HEIGHT}" --anchor-manifest "${ANCHOR}" \
  --output "${A_REPORT}"

recovery_run inspect-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COPY_B}" --read-only \
  --full-state-check --full-offchain-check --require-preimages \
  --target-height "${TARGET_HEIGHT}" --anchor-manifest "${ANCHOR}" \
  --output "${B_REPORT}"

# The agreement verdict is written as a side effect of --compare-with; it
# names both reports by SHA-256. Replay consumes it.
recovery_run inspect-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COPY_A}" --read-only \
  --full-state-check --full-offchain-check --require-preimages \
  --target-height "${TARGET_HEIGHT}" --anchor-manifest "${ANCHOR}" \
  --output "${A_REPORT}" \
  --compare-with "${B_REPORT}" \
  --agreement-output "${RECOVERY_REPORT_DIR}/agreement.json"

echo "inspect: ${A_REPORT}, ${B_REPORT}, agreement ${RECOVERY_REPORT_DIR}/agreement.json"
