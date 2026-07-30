#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR_TEST="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_TEST"' EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local needle=$1 file=$2
  grep -F -- "$needle" "$file" >/dev/null || fail "expected '$needle' in $file"
}

assert_not_contains() {
  local needle=$1 file=$2
  if grep -F -- "$needle" "$file" >/dev/null; then
    fail "did not expect '$needle' in $file"
  fi
}

[[ -x "$ROOT/test/localnet.sh" ]] || fail "test/localnet.sh must be executable"
[[ -x "$ROOT/test/copy_harmony_worktree.sh" ]] || fail "copy helper must be executable"
[[ -f "$ROOT/test/localnet.Dockerfile" ]] || fail "pinned localnet Dockerfile must exist"
assert_not_contains "docker pull" "$ROOT/test/all.sh"
assert_not_contains "docker pull" "$ROOT/test/go.sh"
assert_not_contains "docker pull" "$ROOT/test/rpc.sh"
assert_not_contains "docker pull" "$ROOT/test/rosetta.sh"
assert_not_contains "docker" "$ROOT/test/go.sh"
assert_contains 'localnet.sh" rpc rosetta pyhmy' "$ROOT/test/all.sh"
assert_contains '6afe7cdc1ecdb920d1c9a19d1b1ca912d3a590ab' "$ROOT/test/localnet.sh"
assert_contains 'test: test-go' "$ROOT/Makefile"
assert_contains 'test-integration:' "$ROOT/Makefile"
assert_contains 'test-all:' "$ROOT/Makefile"

bash "$ROOT/test/rpc.sh" >/dev/null || fail "rpc.sh without arguments must show usage"
bash "$ROOT/test/rosetta.sh" >/dev/null || fail "rosetta.sh without arguments must show usage"

mkdir -p "$TMPDIR_TEST/source"
git -C "$TMPDIR_TEST/source" init -q
git -C "$TMPDIR_TEST/source" config user.email test@example.com
git -C "$TMPDIR_TEST/source" config user.name test
printf 'keep\n' > "$TMPDIR_TEST/source/keep"
printf 'delete\n' > "$TMPDIR_TEST/source/deleted"
git -C "$TMPDIR_TEST/source" add keep deleted
git -C "$TMPDIR_TEST/source" commit -qm fixture
rm "$TMPDIR_TEST/source/deleted"
printf 'untracked\n' > "$TMPDIR_TEST/source/untracked"
"$ROOT/test/copy_harmony_worktree.sh" "$TMPDIR_TEST/source" "$TMPDIR_TEST/copied"
[[ -f "$TMPDIR_TEST/copied/keep" ]] || fail "tracked files must be copied"
[[ -f "$TMPDIR_TEST/copied/untracked" ]] || fail "untracked files must be copied"
[[ ! -e "$TMPDIR_TEST/copied/deleted" ]] || fail "tracked deletions must be preserved"

mkdir -p "$TMPDIR_TEST/bin" "$TMPDIR_TEST/harmony-test/localnet" "$TMPDIR_TEST/run-root"
: > "$TMPDIR_TEST/harmony-test/localnet/Dockerfile"
cat > "$TMPDIR_TEST/bin/docker" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$DOCKER_LOG"
if [[ ${DOCKER_FAIL_RUN:-} == 1 && ${1:-} == run ]]; then
  exit 42
fi
STUB
chmod +x "$TMPDIR_TEST/bin/docker"

cat > "$TMPDIR_TEST/bin/build-linux-binaries" <<'STUB'
#!/usr/bin/env bash
printf 'built\n' >> "$BUILDER_LOG"
STUB
chmod +x "$TMPDIR_TEST/bin/build-linux-binaries"

DOCKER_LOG="$TMPDIR_TEST/docker.log" \
BUILDER_LOG="$TMPDIR_TEST/builder.log" \
HARMONY_TEST_DIR="$TMPDIR_TEST/harmony-test" \
HARMONY_BINARY_BUILDER="$TMPDIR_TEST/bin/build-linux-binaries" \
HARMONY_RUN_ROOT="$TMPDIR_TEST/run-root" \
LOCALNET_ARCH=arm64 \
LOCALNET_IMAGE="harmony-localnet-test:test" \
PATH="$TMPDIR_TEST/bin:$PATH" \
  "$ROOT/test/localnet.sh" rpc rosetta pyhmy

[[ $(grep -c '^built$' "$TMPDIR_TEST/builder.log") -eq 1 ]] || fail "Linux binaries must be prepared once"
[[ $(grep -c '^build ' "$TMPDIR_TEST/docker.log") -eq 1 ]] || fail "image must be built exactly once"
[[ $(grep -c '^run ' "$TMPDIR_TEST/docker.log") -eq 3 ]] || fail "integration suites must use isolated runs"
assert_contains "build --pull" "$TMPDIR_TEST/docker.log"
assert_contains "--platform linux/arm64" "$TMPDIR_TEST/docker.log"
assert_contains "-f $ROOT/test/localnet.Dockerfile" "$TMPDIR_TEST/docker.log"
assert_contains "-t harmony-localnet-test:test" "$TMPDIR_TEST/docker.log"
assert_contains "harmony-localnet-test:test -B -n" "$TMPDIR_TEST/docker.log"
assert_contains "harmony-localnet-test:test -B -p" "$TMPDIR_TEST/docker.log"
assert_contains "harmony-localnet-test:test -B -r" "$TMPDIR_TEST/docker.log"
assert_not_contains "pull harmonyone/localnet-test" "$TMPDIR_TEST/docker.log"
assert_contains 'GOFLAGS=-buildvcs=false -mod=mod' "$ROOT/test/build_linux_binaries.sh"
assert_contains 'FROM golang:1.24.2@sha256:' "$ROOT/test/localnet.Dockerfile"
assert_contains 'PYHMY_REF=5aeb8601fa174c734f9091619520cf3160b04a16' "$ROOT/test/localnet.Dockerfile"
assert_contains 'MESH_CLI_REF=bbbd759336a0912853d8b10e84a3731c7a0b99d3' "$ROOT/test/localnet.Dockerfile"
assert_contains 'setuptools==80.9.0' "$ROOT/test/localnet.Dockerfile"
assert_contains '--no-build-isolation' "$ROOT/test/localnet.Dockerfile"

: > "$TMPDIR_TEST/docker.log"
DOCKER_LOG="$TMPDIR_TEST/docker.log" \
BUILDER_LOG="$TMPDIR_TEST/builder.log" \
HARMONY_TEST_DIR="$TMPDIR_TEST/harmony-test" \
HARMONY_BINARY_BUILDER="$TMPDIR_TEST/bin/build-linux-binaries" \
HARMONY_RUN_ROOT="$TMPDIR_TEST/run-root" \
LOCALNET_ARCH=arm64 \
PATH="$TMPDIR_TEST/bin:$PATH" \
  "$ROOT/test/localnet.sh" rpc
assert_contains "image rm -f harmony-localnet-test:" "$TMPDIR_TEST/docker.log"

: > "$TMPDIR_TEST/docker.log"
DOCKER_LOG="$TMPDIR_TEST/docker.log" \
BUILDER_LOG="$TMPDIR_TEST/builder.log" \
HARMONY_TEST_DIR="$TMPDIR_TEST/harmony-test" \
HARMONY_BINARY_BUILDER="$TMPDIR_TEST/bin/build-linux-binaries" \
HARMONY_RUN_ROOT="$TMPDIR_TEST/run-root" \
LOCALNET_ARCH=arm64 \
PATH="$TMPDIR_TEST/bin:$PATH" \
  "$ROOT/test/localnet.sh" --keep rpc
assert_contains "-t harmony-localnet-test:keep" "$TMPDIR_TEST/docker.log"
assert_contains "--name harmony-localnet-test" "$TMPDIR_TEST/docker.log"
assert_contains "image rm -f harmony-localnet-test:keep" "$TMPDIR_TEST/docker.log"

: > "$TMPDIR_TEST/docker.log"
if DOCKER_FAIL_RUN=1 \
  DOCKER_LOG="$TMPDIR_TEST/docker.log" \
  BUILDER_LOG="$TMPDIR_TEST/builder.log" \
  HARMONY_TEST_DIR="$TMPDIR_TEST/harmony-test" \
  HARMONY_BINARY_BUILDER="$TMPDIR_TEST/bin/build-linux-binaries" \
  HARMONY_RUN_ROOT="$TMPDIR_TEST/run-root" \
  LOCALNET_ARCH=arm64 \
  PATH="$TMPDIR_TEST/bin:$PATH" \
    "$ROOT/test/localnet.sh" rpc >/dev/null 2>&1; then
  fail "docker run failures must propagate"
fi
assert_contains "image rm -f harmony-localnet-test:" "$TMPDIR_TEST/docker.log"

if HARMONY_TEST_DIR="$TMPDIR_TEST/harmony-test" PATH="$TMPDIR_TEST/bin:$PATH" \
  "$ROOT/test/localnet.sh" unsupported >/dev/null 2>&1; then
  fail "unknown mode must fail"
fi

echo "localnet runner tests passed"
