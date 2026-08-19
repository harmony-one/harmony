#!/usr/bin/env bash
# Phase E: deep read-only verification of the compact artifact (plan WS6).
# The mode must match compact.json (supply METADATA_REFERENCE_MANIFEST iff the
# build was reference mode).
#
#   RECOVERY_REPORT_DIR=/reports \
#   COMPACT_DB=/data/compact/harmony_db_0 ANCHOR=/reports/anchor.json \
#   scripts/recovery/55-verify.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require COMPACT_DB "${COMPACT_DB:-}"
recovery_require ANCHOR "${ANCHOR:-}"

REF_FLAG=()
if [[ -n "${METADATA_REFERENCE_MANIFEST:-}" ]]; then
  REF_FLAG=(--metadata-reference-manifest "${METADATA_REFERENCE_MANIFEST}")
fi

recovery_run verify-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COMPACT_DB}" --read-only \
  --anchor-manifest "${ANCHOR}" \
  --full-state-check --full-offchain-check \
  --source-reference "${RECOVERY_REPORT_DIR}/compact.json" \
  "${REF_FLAG[@]}" \
  --output "${RECOVERY_REPORT_DIR}/verification.json"

echo "verify: ${RECOVERY_REPORT_DIR}/verification.json"
