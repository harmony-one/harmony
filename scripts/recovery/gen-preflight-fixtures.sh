#!/usr/bin/env bash
# Regenerates the deterministic preflight fixtures under
# testdata/recovery/preflight/. The fixtures embed fixture-only BLS secrets
# (small fixed scalars) and are safe to commit; they must never be used
# outside tests. Requires the BLS build environment
# (scripts/setup_bls_build_flags.sh).
set -euo pipefail

cd "$(dirname "${0}")/../.."
. scripts/setup_bls_build_flags.sh

go run ./internal/recovery/inplace/fixture/gen "${1:-testdata/recovery/preflight}"
echo "fixtures written to ${1:-testdata/recovery/preflight}"
