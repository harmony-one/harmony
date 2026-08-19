#!/usr/bin/env bash
# Materialize the localnet fixture kit on disk (plan WS9). The chains carry
# real BLS certificates from the public dev keys in .hmy — no secrets. The Go
# test suite generates its own kit in-process; this wrapper is for manual and
# dry-run use (e.g. e2e-localnet.sh, INSTALL.md rehearsals).
#
#   OUT=/tmp/recovery-fixtures scripts/recovery/gen-fixtures.sh

set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="${OUT:-/tmp/recovery-fixtures}"

# shellcheck disable=SC1091
source "${ROOT}/scripts/recovery/common.sh"

mkdir -p "${OUT}"
recovery_gorun ./internal/recoverydb/fixture/gen \
    --out "${OUT}" --keys "${ROOT}/.hmy" \
    --baseline "${BASELINE:-18}" --target "${TARGET:-22}" --donor "${DONOR:-26}"

echo "fixture kit: ${OUT}"
