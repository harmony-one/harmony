#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HARMONY_TEST_REPOSITORY=${HARMONY_TEST_REPOSITORY:-https://github.com/harmony-one/harmony-test.git}
HARMONY_TEST_REF=${HARMONY_TEST_REF:-6afe7cdc1ecdb920d1c9a19d1b1ca912d3a590ab}
HARMONY_BINARY_BUILDER=${HARMONY_BINARY_BUILDER:-$ROOT/test/build_linux_binaries.sh}
MAIN_REPO_ORG=${MAIN_REPO_ORG:-harmony-one}
MAIN_REPO_BRANCH=${MAIN_REPO_BRANCH:-$(git -C "$ROOT" branch --show-current)}
KEEP=false
MODES=()
TEMP_TEST_DIR=
TEMP_RUN_DIR=
REMOVE_IMAGE=false
IMAGE_BUILT=false
IMAGE_EXPLICIT=false
if [[ -n ${LOCALNET_IMAGE:-} ]]; then
  IMAGE_EXPLICIT=true
fi

usage() {
  cat <<'EOF'
Usage: test/localnet.sh [--keep] MODE [MODE...]

Modes:
  rpc      Run RPC integration tests
  pyhmy    Run pyhmy tests
  mesh     Run Mesh API integration tests
  rosetta  Alias for mesh

Environment:
  HARMONY_TEST_REF         harmony-test branch, tag, or commit (default: pinned commit)
  HARMONY_TEST_DIR         existing harmony-test checkout; skips the temporary fetch
  HARMONY_TEST_REPOSITORY  repository used when HARMONY_TEST_DIR is unset
  HARMONY_BINARY_BUILDER   prepares compatible Linux harmony/bootnode binaries
  HARMONY_RUN_ROOT         existing disposable Harmony tree; skips temporary copy
  LOCALNET_ARCH            Docker architecture (default: Docker daemon architecture)
  LOCALNET_IMAGE           image tag to retain; unset uses a temporary local tag
EOF
}

cleanup() {
  local run_status=$?

  if [[ -n "$TEMP_RUN_DIR" ]]; then
    if [[ "$IMAGE_BUILT" == true ]]; then
      docker run --rm \
        --platform "linux/$LOCALNET_ARCH" \
        --entrypoint /bin/sh \
        -v "$RUN_ROOT:/cleanup" \
        "$LOCALNET_IMAGE" \
        -c 'find /cleanup -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +' \
        >/dev/null 2>&1 || true
    fi
    rm -rf "$TEMP_RUN_DIR" 2>/dev/null || \
      echo "[WARN] unable to remove temporary run directory: $TEMP_RUN_DIR" >&2
  fi
  if [[ -n "$TEMP_TEST_DIR" ]]; then
    rm -rf "$TEMP_TEST_DIR"
  fi
  if [[ "$REMOVE_IMAGE" == true && "$IMAGE_BUILT" == true && "$KEEP" != true ]]; then
    docker image rm -f "$LOCALNET_IMAGE" >/dev/null 2>&1 || true
  fi

  return "$run_status"
}
trap cleanup EXIT

while (($#)); do
  case "$1" in
    --keep) KEEP=true ;;
    rpc|pyhmy|mesh|rosetta) MODES+=("$1") ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown localnet test mode: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if ((${#MODES[@]} == 0)); then
  usage >&2
  exit 2
fi
if [[ "$KEEP" == true && ${#MODES[@]} -ne 1 ]]; then
  echo "--keep requires exactly one test mode" >&2
  exit 2
fi

LOCALNET_ARCH=${LOCALNET_ARCH:-$(docker info --format '{{.Architecture}}')}
case "$LOCALNET_ARCH" in
  arm64|aarch64) LOCALNET_ARCH=arm64 ;;
  amd64|x86_64) LOCALNET_ARCH=amd64 ;;
  *) echo "Unsupported Docker architecture: $LOCALNET_ARCH" >&2; exit 2 ;;
esac
export LOCALNET_ARCH

"$HARMONY_BINARY_BUILDER"

if [[ -n ${HARMONY_TEST_DIR:-} ]]; then
  TEST_DIR=$(cd "$HARMONY_TEST_DIR" && pwd)
  echo "[harmony-test] using local checkout: $TEST_DIR"
else
  TEMP_TEST_DIR=$(mktemp -d)
  TEST_DIR="$TEMP_TEST_DIR/harmony-test"
  git init -q "$TEST_DIR"
  git -C "$TEST_DIR" remote add origin "$HARMONY_TEST_REPOSITORY"
  git -C "$TEST_DIR" fetch --depth=1 origin "$HARMONY_TEST_REF"
  git -C "$TEST_DIR" checkout -q --detach FETCH_HEAD
  echo "[harmony-test] using ref '$HARMONY_TEST_REF' at $(git -C "$TEST_DIR" rev-parse HEAD)"
fi

if [[ ! -f "$TEST_DIR/localnet/Dockerfile" ]]; then
  echo "Missing harmony-test localnet Dockerfile: $TEST_DIR/localnet/Dockerfile" >&2
  exit 1
fi

if [[ -z ${LOCALNET_IMAGE:-} && "$KEEP" == true ]]; then
  LOCALNET_IMAGE=harmony-localnet-test:keep
elif [[ -z ${LOCALNET_IMAGE:-} ]]; then
  resolved_ref=$(git -C "$TEST_DIR" rev-parse --short=12 HEAD 2>/dev/null || echo local)
  LOCALNET_IMAGE="harmony-localnet-test:${resolved_ref}-$$"
  REMOVE_IMAGE=true
fi

if [[ "$KEEP" == true ]]; then
  docker rm -f harmony-localnet-test >/dev/null 2>&1 || true
  if [[ "$IMAGE_EXPLICIT" != true ]]; then
    docker image rm -f "$LOCALNET_IMAGE" >/dev/null 2>&1 || true
  fi
fi

docker build --pull \
  --platform "linux/$LOCALNET_ARCH" \
  -f "$ROOT/test/localnet.Dockerfile" \
  --build-arg "MAIN_REPO_BRANCH=$MAIN_REPO_BRANCH" \
  --build-arg "MAIN_REPO_ORG=$MAIN_REPO_ORG" \
  --progress plain \
  -t "$LOCALNET_IMAGE" \
  "$TEST_DIR/localnet"
IMAGE_BUILT=true

if [[ -n ${HARMONY_RUN_ROOT:-} ]]; then
  RUN_ROOT=$(cd "$HARMONY_RUN_ROOT" && pwd)
else
  TEMP_RUN_DIR=$(mktemp -d)
  RUN_ROOT="$TEMP_RUN_DIR/harmony"
  "$ROOT/test/copy_harmony_worktree.sh" "$ROOT" "$RUN_ROOT"
  mkdir -p "$RUN_ROOT/bin"
  cp "$ROOT/bin/harmony" "$ROOT/bin/bootnode" "$RUN_ROOT/bin/"
fi

PORTS=(
  -p 9500:9500 -p 9501:9501
  -p 9599:9599 -p 9598:9598
  -p 9799:9799 -p 9798:9798
  -p 9899:9899 -p 9898:9898
)

for mode in "${MODES[@]}"; do
  case "$mode" in
    rpc) flags=(-B -n) ;;
    pyhmy) flags=(-B -p) ;;
    mesh|rosetta) flags=(-B -r) ;;
  esac

  run_args=(run)
  if [[ "$KEEP" == true ]]; then
    run_args+=(--name harmony-localnet-test)
    flags+=(-k)
  else
    run_args+=(--rm)
  fi

  docker "${run_args[@]}" \
    --platform "linux/$LOCALNET_ARCH" \
    -e HARMONY_PREBUILT_STATIC=true \
    "${PORTS[@]}" \
    -v "$RUN_ROOT:/go/src/github.com/harmony-one/harmony" \
    "$LOCALNET_IMAGE" "${flags[@]}"
done
