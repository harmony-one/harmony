#!/usr/bin/env bash
# Phase C: strict offline replay into the working copy up to the pinned target
# (plan WS4). The destination is the designated Aug 8 working copy; v1 never
# resumes an unclean destination.
#
#   RECOVERY_REPORT_DIR=/reports \
#   WORKING_DB=/data/aug8-a/harmony_db_0 ANCHOR=/reports/anchor.json \
#   BUNDLE_DIR=/data/bundle MIN_FREE_BYTES=1000000000000 \
#   scripts/recovery/40-replay.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require WORKING_DB "${WORKING_DB:-}"
recovery_require ANCHOR "${ANCHOR:-}"
recovery_require BUNDLE_DIR "${BUNDLE_DIR:-}"

recovery_run replay-bundle --network "${NETWORK}" --shard "${SHARD}" \
  --destination-db "${WORKING_DB}" \
  --inspect-report "${RECOVERY_REPORT_DIR}/inspect-a.json" \
  --baseline-agreement "${RECOVERY_REPORT_DIR}/agreement.json" \
  --bundle "${BUNDLE_DIR}" \
  --anchor-manifest "${ANCHOR}" --target-height "${TARGET_HEIGHT}" \
  --offline --no-resume-on-unclean-exit \
  --min-free-bytes "${MIN_FREE_BYTES:-0}" \
  --output "${RECOVERY_REPORT_DIR}/replay.json"

echo "replay: ${RECOVERY_REPORT_DIR}/replay.json"
