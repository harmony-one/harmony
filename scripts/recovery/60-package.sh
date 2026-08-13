#!/usr/bin/env bash
# Phase F: single-invocation sealer (plan WS7). No upload wrapper — devops
# owns publishing; package-db prints the handoff note.
#
#   RECOVERY_REPORT_DIR=/reports \
#   COMPACT_DB=/data/compact/harmony_db_0 ANCHOR=/reports/anchor.json \
#   RELEASE_ROOT=/data/release \
#   scripts/recovery/60-package.sh

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require COMPACT_DB "${COMPACT_DB:-}"
recovery_require ANCHOR "${ANCHOR:-}"
recovery_require RELEASE_ROOT "${RELEASE_ROOT:-}"

OPT_FLAGS=()
if [[ -n "${RECOVERY_HARMONY_BINARY_SHA256:-}" ]]; then
  OPT_FLAGS+=(--recovery-harmony-binary-sha256 "${RECOVERY_HARMONY_BINARY_SHA256}")
fi
if [[ -n "${PROVISIONAL_START_VIEW_ID:-}" ]]; then
  OPT_FLAGS+=(--provisional-start-view-id "${PROVISIONAL_START_VIEW_ID}")
fi

recovery_run package-db --network "${NETWORK}" --shard "${SHARD}" \
  --db "${COMPACT_DB}" --anchor-manifest "${ANCHOR}" \
  --target-height "${TARGET_HEIGHT}" \
  --verification-report "${RECOVERY_REPORT_DIR}/verification.json" \
  --release-root "${RELEASE_ROOT}" \
  "${OPT_FLAGS[@]}" \
  --output "${RECOVERY_REPORT_DIR}/package.json"

echo "package: ${RECOVERY_REPORT_DIR}/package.json (release under ${RELEASE_ROOT})"
