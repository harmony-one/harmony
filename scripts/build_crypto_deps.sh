#!/usr/bin/env bash

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# shellcheck source=scripts/setup_bls_build_flags.sh
. "${ROOT}/scripts/setup_bls_build_flags.sh"

jobs="${JOBS:-}"
if [[ -z "${jobs}" ]]; then
	if command -v nproc >/dev/null 2>&1; then
		jobs=$(nproc)
	elif command -v sysctl >/dev/null 2>&1; then
		jobs=$(sysctl -n hw.ncpu)
	else
		jobs=4
	fi
fi

if [[ ! -d "${MCL_DIR}" ]]; then
	echo "missing mcl repository: ${MCL_DIR}" >&2
	exit 1
fi

if [[ ! -d "${BLS_DIR}" ]]; then
	echo "missing bls repository: ${BLS_DIR}" >&2
	exit 1
fi

echo "building mcl (${MCL_DIR}) with -j${jobs}"
make -C "${MCL_DIR}" -j"${jobs}"

if [[ "${STATIC_BLS:-false}" == "true" ]]; then
	echo "building bls minimised_static (${BLS_DIR}) with -j${jobs}"
	make -C "${BLS_DIR}" minimised_static BLS_SWAP_G=1 -j"${jobs}"
else
	echo "building bls (${BLS_DIR}) with -j${jobs}"
	make -C "${BLS_DIR}" BLS_SWAP_G=1 -j"${jobs}"
fi
