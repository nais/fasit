#!/usr/bin/env bash
#MISE description="Release naisd"
set -euo pipefail

# If there's changes locally (only tracked files), or the commits aren't pushed, exit with error
if [ -n "$(git status --untracked-files=no --porcelain=v1)" ] || [ -n "$(git log origin/main..HEAD)" ]; then
  echo "There are uncommitted changes or commits not pushed to origin/master"
  exit 1
fi

tag="naisd-$(date +%Y%m%d%H%M%S)"
# If there's no changes locally, tag the release with `naisd-[datetime]` and push it to origin
git tag -a "$tag" -m "Release naisd"

# Push the tag to origin
git push origin --tags

# Delete the tag locally
git tag -d "$tag"
