#!/usr/bin/env bash

set -euo pipefail

main_repo_branch=${1:?missing main repo branch}
main_repo_org=${2:?missing main repo org}
image_tag=${3:-harmonyone/localnet-test}

cache_scope_branch=$(printf "%s" "${main_repo_branch}" | tr -c "[:alnum:]_.-" "-")
cache_scope="harmony-test-localnet-${cache_scope_branch}"

if [[ "${HARMONY_TEST_DOCKER_CACHE:-false}" == "true" ]]; then
	docker buildx build \
		--build-arg MAIN_REPO_BRANCH="${main_repo_branch}" \
		--build-arg MAIN_REPO_ORG="${main_repo_org}" \
		--progress plain \
		--cache-from "type=gha,scope=${cache_scope}" \
		--cache-to "type=gha,mode=max,scope=${cache_scope}" \
		--load \
		-t "${image_tag}" .
else
	docker build \
		--build-arg MAIN_REPO_BRANCH="${main_repo_branch}" \
		--build-arg MAIN_REPO_ORG="${main_repo_org}" \
		--progress plain \
		-t "${image_tag}" .
fi
