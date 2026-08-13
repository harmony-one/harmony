#!/usr/bin/env bash
# Phase D: strict compaction into a fresh validator harmony_db_0 (plan WS5).
# Internal mode by default; set METADATA_REFERENCE_MANIFEST for reference mode.
#
#   RECOVERY_REPORT_DIR=/reports \
#   WORKING_DB=/data/aug8-a/harmony_db_0 COMPACT_DB=/data/compact/harmony_db_0 \
#   ANCHOR=/reports/anchor.json \
#   scripts/recovery/50-compact.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require WORKING_DB "${WORKING_DB:-}"
recovery_require COMPACT_DB "${COMPACT_DB:-}"
recovery_require ANCHOR "${ANCHOR:-}"

REF_FLAG=()
if [[ -n "${METADATA_REFERENCE_MANIFEST:-}" ]]; then
  REF_FLAG=(--metadata-reference-manifest "${METADATA_REFERENCE_MANIFEST}")
fi

recovery_run compact-db --network "${NETWORK}" --shard "${SHARD}" \
  --source-db "${WORKING_DB}" --source-read-only \
  --destination-db "${COMPACT_DB}" \
  --anchor-manifest "${ANCHOR}" \
  --source-reference "${RECOVERY_REPORT_DIR}/replay.json" \
  --target-height "${TARGET_HEIGHT}" \
  --fail-if-destination-nonempty \
  "${REF_FLAG[@]}" \
  --output "${RECOVERY_REPORT_DIR}/compact.json"

echo "compact: ${COMPACT_DB} (report ${RECOVERY_REPORT_DIR}/compact.json)"
