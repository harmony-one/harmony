#!/usr/bin/env bash
set -euo pipefail

SOURCE=${1:?source worktree is required}
DESTINATION=${2:?destination is required}

SOURCE=$(cd "$SOURCE" && pwd)
mkdir -p "$DESTINATION"
DESTINATION=$(cd "$DESTINATION" && pwd)

(
  cd "$SOURCE"
  git ls-files -co --exclude-standard -z |
    while IFS= read -r -d '' path; do
      if [[ -e "$path" || -L "$path" ]]; then
        printf '%s\0' "$path"
      fi
    done |
    tar --null -T - -cf -
) | tar -xf - -C "$DESTINATION"
