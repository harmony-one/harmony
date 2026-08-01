#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_BUILDER_IMAGE=${GO_BUILDER_IMAGE:-golang:1.24.2@sha256:30baaea08c5d1e858329c50f29fe381e9b7d7bced11a0f5f1f69a1504cdfbf5e}
LOCALNET_ARCH=${LOCALNET_ARCH:-$(docker info --format '{{.Architecture}}')}
BUILD_STAMP="$ROOT/bin/.localnet-build"

case "$LOCALNET_ARCH" in
  arm64|aarch64)
    LOCALNET_ARCH=arm64
    FILE_ARCH='ARM aarch64'
    ;;
  amd64|x86_64)
    LOCALNET_ARCH=amd64
    FILE_ARCH='x86-64'
    ;;
  *)
    echo "Unsupported localnet architecture: $LOCALNET_ARCH" >&2
    exit 2
    ;;
esac

hash_stream() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | cut -d' ' -f1
  else
    shasum -a 256 | cut -d' ' -f1
  fi
}

source_id=$({
  git -C "$ROOT" rev-parse HEAD
  git -C "$ROOT" diff --binary HEAD -- . ':(exclude)bin'
  while IFS= read -r -d '' path; do
    printf '%s\0' "$path"
    git -C "$ROOT" hash-object "$path"
  done < <(git -C "$ROOT" ls-files --others --exclude-standard -z)
} | hash_stream)
expected_stamp="$source_id $LOCALNET_ARCH $GO_BUILDER_IMAGE"

binaries_match_arch() {
  local binary description
  for binary in harmony bootnode; do
    [[ -x "$ROOT/bin/$binary" ]] || return 1
    description=$(file "$ROOT/bin/$binary")
    [[ "$description" == *"ELF 64-bit"* && "$description" == *"$FILE_ARCH"* ]] || return 1
  done
}

has_current_binaries() {
  binaries_match_arch || return 1
  [[ -f "$BUILD_STAMP" ]] || return 1
  [[ $(<"$BUILD_STAMP") == "$expected_stamp" ]]
}

if has_current_binaries; then
  echo "[harmony] using existing Linux $LOCALNET_ARCH binaries"
  exit 0
fi

git_common_dir=$(git -C "$ROOT" rev-parse --path-format=absolute --git-common-dir)
mkdir -p "$ROOT/bin"

mounts=(
  -v "$ROOT:$ROOT"
  -v harmony-go-mod-cache:/go/pkg/mod
  -v harmony-go-build-cache:/root/.cache/go-build
)
if [[ "$git_common_dir" != "$ROOT"/* ]]; then
  mounts+=(-v "$git_common_dir:$git_common_dir:ro")
fi

echo "[harmony] building Linux $LOCALNET_ARCH binaries with $GO_BUILDER_IMAGE"
docker run --rm \
  --platform "linux/$LOCALNET_ARCH" \
  "${mounts[@]}" \
  -w "$ROOT" \
  -e "GOFLAGS=-buildvcs=false -mod=mod" \
  "$GO_BUILDER_IMAGE" \
  bash ./scripts/go_executable_build.sh -s

if ! binaries_match_arch; then
  echo "Linux $LOCALNET_ARCH harmony/bootnode build did not produce compatible binaries" >&2
  exit 1
fi

printf '%s\n' "$expected_stamp" > "$BUILD_STAMP"
