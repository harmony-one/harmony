#!/usr/bin/env bash
# Regenerates the metadata acceptance fixture chain under
# testdata/recovery/metadata/kit (a localnet twin chain with real BLS
# certificates from the public .hmy dev keys, deterministic block times, and
# fixture-only BLS validator secrets — byte-reproducible, safe to commit,
# never used outside tests). Requires the BLS build environment
# (scripts/setup_bls_build_flags.sh).
#
# The acceptance suite (internal/recovery/metadata/acceptance) regenerates
# fixtures in-process, so this script is for the devops pilot and manual
# inspection. See docs/recovery/metadata.md for the run-once runbook.
set -euo pipefail

cd "$(dirname "${0}")/../.."
# setup_bls_build_flags.sh reads $1 unguarded (its -v flag); relax nounset
# around the source so running this script without arguments works.
set +u
. scripts/setup_bls_build_flags.sh
set -u

OUT="${1:-testdata/recovery/metadata/kit}"

case "$(uname -s)" in
Darwin)
   # Homebrew gmp is not on the default linker path (bls links -lgmp).
   GMP_DIR="${GMP_DIR:-$(brew --prefix gmp 2>/dev/null || echo /opt/homebrew/opt/gmp)}"
   export CGO_LDFLAGS="${CGO_LDFLAGS} -L${GMP_DIR}/lib"
   # SIP strips DYLD_* from the environment across protected binaries
   # (bash/go), so `go run` aborts loading libbls384_256.dylib. Build the
   # generator, then launch it via /usr/bin/env with the variable set
   # explicitly for the (unprotected) generator binary itself — the same
   # mechanism the test suite uses (go test -exec "/usr/bin/env DYLD_...").
   BIN="$(mktemp -d)/metadata-fixture-gen"
   trap 'rm -rf "$(dirname "${BIN}")"' EXIT
   go build -o "${BIN}" ./internal/recovery/metadata/fixture/gen
   /usr/bin/env DYLD_FALLBACK_LIBRARY_PATH="${DYLD_FALLBACK_LIBRARY_PATH}" "${BIN}" "${OUT}"
   ;;
*)
   go run ./internal/recovery/metadata/fixture/gen "${OUT}"
   ;;
esac
echo "metadata fixture kit written to ${OUT}"
