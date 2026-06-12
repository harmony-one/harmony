#!/usr/bin/env bash
set -e

# handle for the Travis build run:
# * uses TRAVIS_PULL_REQUEST_SLUG if PR is done from fork
# * uses TRAVIS_PULL_REQUEST_BRANCH for RP branch
# * uses TRAVIS_BRANCH for simple branch builds
if [[ -z ${TRAVIS_PULL_REQUEST_SLUG} ]]; then
    MAIN_REPO_ORG='harmony-one'
else
    MAIN_REPO_ORG=${TRAVIS_PULL_REQUEST_SLUG%/*}
    echo "[WARN] - working on the fork - ${MAIN_REPO_ORG}"
fi

MAIN_REPO_BRANCH=${TRAVIS_PULL_REQUEST_BRANCH:-${TRAVIS_BRANCH}}
# handle for the local run, covers:
# * branch exist on remote - will use it in the tests
# * branch exists locally - will use dev as base branch in test
if [[ -z "$MAIN_REPO_BRANCH" ]]; then
    MAIN_REPO_BRANCH=${MAIN_REPO_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}
    git ls-remote --exit-code --heads origin "${MAIN_REPO_BRANCH}" >/dev/null 2>&1 || EXIT_CODE=$?
    if [[ $EXIT_CODE == '0' ]]; then
        echo "[INFO] - Git branch '$MAIN_REPO_BRANCH' exists in the remote repository"
    elif [[ $EXIT_CODE == '2' ]]; then
        echo "[WARN] - Git branch '$MAIN_REPO_BRANCH' does not exist in the remote repository, using" \
            "'dev' branch as a workaround for a local-only branch"
        MAIN_REPO_BRANCH='dev'
    fi
fi

echo "[harmony repo] - working on '${MAIN_REPO_BRANCH}' branch"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
echo "Working dir is ${DIR}"
echo "GOPATH is ${GOPATH}"

function timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

function timed() {
    local label=$1
    shift
    local start end elapsed
    start=$(date +%s)
    echo "[TIMING] $(timestamp) START ${label}"
    "$@"
    end=$(date +%s)
    elapsed=$((end - start))
    echo "[TIMING] $(timestamp) END ${label}: ${elapsed}s"
}

function build_localnet_test_image() {
    local cache_scope
    cache_scope="localnet-test-${TEST_REPO_BRANCH//[^a-zA-Z0-9_.-]/-}"
    if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
        docker buildx build --load --build-arg MAIN_REPO_BRANCH="${MAIN_REPO_BRANCH}" --progress plain \
            --build-arg MAIN_REPO_ORG="${MAIN_REPO_ORG}" -t harmonyone/localnet-test \
            --cache-from "type=gha,scope=${cache_scope}" \
            --cache-to "type=gha,mode=max,scope=${cache_scope}" .
    else
        docker build --build-arg MAIN_REPO_BRANCH="${MAIN_REPO_BRANCH}" --progress plain \
            --build-arg MAIN_REPO_ORG="${MAIN_REPO_ORG}" -t harmonyone/localnet-test .
    fi
}

cd "${GOPATH}/src/github.com/harmony-one/harmony-test"
# Current solution expects that your harmony-test repo branch with the same name exists
# or it will use master by default
TEST_REPO_BRANCH=${MAIN_REPO_BRANCH}
# fallback to master if branch is not on remote
TEST_REPO_BRANCH=$(git ls-remote --exit-code --heads origin "${TEST_REPO_BRANCH}" >/dev/null 2>&1 \
    && echo "${TEST_REPO_BRANCH}" || echo "master")
echo "[harmony-test repo] - working on '${TEST_REPO_BRANCH}' branch"
# cover possible force pushes to remote branches - just rebase local on top of origin
timed "harmony-test fetch" git fetch origin "${TEST_REPO_BRANCH}"
timed "harmony-test checkout" git checkout "${TEST_REPO_BRANCH}"
timed "harmony-test pull" git pull --rebase=true
cd localnet
timed "docker build localnet-test" build_localnet_test_image
# WARN: this is the place where LOCAL repository is provided to the harmony-tests repo
timed "docker run rpc tests" docker run \
    -e PYTEST_ADDOPTS="${PYTEST_ADDOPTS:---durations=25 --durations-min=1}" \
    -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -n
