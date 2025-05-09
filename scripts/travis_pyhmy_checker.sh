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
cd "${GOPATH}/src/github.com/harmony-one/harmony-test"
# Current solution expects that your harmony-test repo branch with the same name exists
# or it will use master by default
TEST_REPO_BRANCH=${MAIN_REPO_BRANCH}
# fallback to master if branch is not on remote
TEST_REPO_BRANCH=$(git ls-remote --exit-code --heads origin "${TEST_REPO_BRANCH}" >/dev/null 2>&1 \
    && echo "${TEST_REPO_BRANCH}" || echo "master")
echo "[harmony-test repo] - working on '${TEST_REPO_BRANCH}' branch"
# cover possible force pushes to remote branches - just rebase local on top of origin
git fetch origin "${TEST_REPO_BRANCH}"
git checkout "${TEST_REPO_BRANCH}"
git pull --rebase=true
cd localnet
docker build --build-arg MAIN_REPO_BRANCH="${MAIN_REPO_BRANCH}" --progress plain \
    --build-arg MAIN_REPO_ORG="${MAIN_REPO_ORG}" -t harmonyone/localnet-test .
# WARN: this is the place where LOCAL repository is provided to the harmony-tests repo
docker run -v "$DIR/../:/go/src/github.com/harmony-one/harmony" harmonyone/localnet-test -p
