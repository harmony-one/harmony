#!/usr/bin/env bash

set -euo pipefail

main_repo_branch=${1:?missing main repo branch}
main_repo_org=${2:?missing main repo org}
image_tag=${3:-harmonyone/localnet-test}

cache_scope=${HARMONY_TEST_DOCKER_CACHE_SCOPE:-harmony-test-localnet}

if [[ "${HARMONY_TEST_DOCKER_CACHE:-false}" == "true" ]]; then
	echo "Building ${image_tag} with Docker Buildx cache scope: ${cache_scope}"
	docker buildx build \
		--build-arg MAIN_REPO_BRANCH="${main_repo_branch}" \
		--build-arg MAIN_REPO_ORG="${main_repo_org}" \
		--progress plain \
		--cache-from "type=gha,scope=${cache_scope}" \
		--cache-to "type=gha,mode=max,scope=${cache_scope}" \
		--load \
		-t "${image_tag}" .
else
	echo "Building ${image_tag} without Docker Buildx cache"
	docker build \
		--build-arg MAIN_REPO_BRANCH="${main_repo_branch}" \
		--build-arg MAIN_REPO_ORG="${main_repo_org}" \
		--progress plain \
		-t "${image_tag}" .
fi
