#!/usr/bin/env bash

set -euo pipefail

test_mode=${1:?missing localnet test mode, expected -n, -p, or -r}
image_tag=${HARMONY_TEST_IMAGE:-harmonyone/localnet-test}
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

echo "Pulling ${image_tag}"
docker pull "${image_tag}"

echo "Running ${image_tag} ${test_mode}"
docker run \
	-v "${root}:/go/src/github.com/harmony-one/harmony" \
	"${image_tag}" "${test_mode}"
