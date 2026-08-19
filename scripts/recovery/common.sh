#!/usr/bin/env bash
# Shared helpers for the harmony-recovery-db operator wrappers (plan WS9).
# Producer-side only. Wrappers refuse to run without an explicit
# RECOVERY_REPORT_DIR and never embed secrets.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Locate the built binary; fall back to `go run` for dev use.
BIN="${HARMONY_RECOVERY_DB_BIN:-${ROOT}/bin/harmony-recovery-db}"
if [[ ! -x "${BIN}" ]]; then
  BIN=""
fi

recovery_require_report_dir() {
  if [[ -z "${RECOVERY_REPORT_DIR:-}" ]]; then
    echo "error: set RECOVERY_REPORT_DIR to a durable reports directory" >&2
    exit 2
  fi
  mkdir -p "${RECOVERY_REPORT_DIR}"
}

recovery_require() {
  local name="$1" val="${2:-}"
  if [[ -z "${val}" ]]; then
    echo "error: ${name} is required" >&2
    exit 2
  fi
}

# recovery_build_env sources the repo BLS build flags and repairs the two
# gaps that break fresh Darwin shells (round 13 finding 11): libmcl/libbls
# depend on gmp, but setup_bls_build_flags.sh adds gmp to neither the LINK
# path (CGO_LDFLAGS/LIBRARY_PATH — `ld: library 'gmp' not found`) nor the
# LOAD path (DYLD_FALLBACK_LIBRARY_PATH). GOTOOLCHAIN=auto lets go pick the
# go.mod-required toolchain on hosts pinned to an older local one.
recovery_build_env() {
  # The explicit empty positional arg keeps the sourced script's `$1` probe
  # defined under this file's `set -u` (bash gives sourced files the source
  # command's args, else the caller's).
  # shellcheck disable=SC1091
  source "${ROOT}/scripts/setup_bls_build_flags.sh" ""
  export GOTOOLCHAIN="${GOTOOLCHAIN:-auto}"
  local gmp
  for gmp in /opt/homebrew/opt/gmp/lib /usr/local/opt/gmp/lib; do
    if [[ -d "${gmp}" ]]; then
      export CGO_LDFLAGS="${CGO_LDFLAGS} -L${gmp}"
      export LIBRARY_PATH="${LIBRARY_PATH:+${LIBRARY_PATH}:}${gmp}"
      export LD_LIBRARY_PATH="${LD_LIBRARY_PATH}:${gmp}"
      break
    fi
  done
}

# recovery_gorun runs `go run PKG ARGS...`, re-injecting the BLS loader path
# through `env` right before exec so macOS SIP does not strip DYLD_* across
# the go-run child. On Linux the env wrapper is a harmless no-op.
recovery_gorun() {
  recovery_build_env
  ( cd "${ROOT}" && go run -exec "/usr/bin/env DYLD_FALLBACK_LIBRARY_PATH=${LD_LIBRARY_PATH}" "$@" )
}

# recovery_run runs the tool, either the built binary or `go run` (dev
# fallback).
recovery_run() {
  if [[ -n "${BIN}" ]]; then
    "${BIN}" "$@"
  else
    recovery_gorun ./cmd/harmony-recovery-db "$@"
  fi
}

NETWORK="${NETWORK:-mainnet}"
SHARD="${SHARD:-0}"
TARGET_HEIGHT="${TARGET_HEIGHT:-92730034}"
