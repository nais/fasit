#!/usr/bin/env bash
#MISE description="Upgrade all github actions to latest version satisfying their version tag"
set -euo pipefail

if [ -z "${GITHUB_TOKEN:-}" ]; then
  if command -v gh >/dev/null 2>&1; then
    GITHUB_TOKEN="$(gh auth token)"
    export GITHUB_TOKEN
  else
    echo "Error: GITHUB_TOKEN environment variable is required to avoid API rate limits."
    echo "Please log in with: gh auth login"
    exit 1
  fi
fi

go tool github.com/sethvargo/ratchet update .github/workflows/*.yaml
