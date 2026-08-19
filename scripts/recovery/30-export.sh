#!/usr/bin/env bash
# Phase B: single-donor bundle export (plan WS3). Pass --report-only (or set
# REPORT_ONLY=1) for the mechanical donor preflight. Export from a STOPPED
# donor copy or a crash-consistent snapshot — never a live directory.
#
#   RECOVERY_REPORT_DIR=/reports \
#   DONOR=/data/donor/harmony_db_0 BASELINE_REPORT=/reports/inspect-a.json \
#   ANCHOR=/reports/anchor.json BUNDLE_DIR=/data/bundle \
#   FROM_HEIGHT=92591098 CERT_CHILD_HEIGHT=92730035 \
#   scripts/recovery/30-export.sh [--report-only]

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
recovery_require_report_dir
recovery_require DONOR "${DONOR:-}"
recovery_require BASELINE_REPORT "${BASELINE_REPORT:-}"
recovery_require ANCHOR "${ANCHOR:-}"
recovery_require FROM_HEIGHT "${FROM_HEIGHT:-}"
recovery_require CERT_CHILD_HEIGHT "${CERT_CHILD_HEIGHT:-}"

REPORT_ONLY_FLAG=""
if [[ "${REPORT_ONLY:-0}" == "1" || "${1:-}" == "--report-only" ]]; then
  REPORT_ONLY_FLAG="--report-only"
  OUT="${RECOVERY_REPORT_DIR}/export-preflight.json"
else
  recovery_require BUNDLE_DIR "${BUNDLE_DIR:-}"
  OUT="${BUNDLE_DIR}"
fi

recovery_run export-bundle --network "${NETWORK}" --shard "${SHARD}" \
  --source-db "${DONOR}" --read-only \
  --baseline-manifest "${BASELINE_REPORT}" \
  --from-height "${FROM_HEIGHT}" --to-height "${TARGET_HEIGHT}" \
  --certificate-child-height "${CERT_CHILD_HEIGHT}" \
  --anchor-manifest "${ANCHOR}" \
  --donor "${DONOR_ID:-mhe-snaps0-01}" \
  ${REPORT_ONLY_FLAG} \
  --output "${OUT}"

echo "export: ${OUT}"
