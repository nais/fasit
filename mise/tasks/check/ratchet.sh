#!/usr/bin/env bash
#MISE description="Verify all github actions are pinned"
set -euo pipefail

GITHUB_TOKEN="${GITHUB_TOKEN:-${MISE_GITHUB_TOKEN:-}}"

if [ -z "$GITHUB_TOKEN" ]; then
  if command -v gh >/dev/null 2>&1; then
    GITHUB_TOKEN="$(gh auth token)"
  else
    echo "Warning: GITHUB_TOKEN not set. You may hit API rate limits."
    echo "Please log in with: gh auth login"
    echo ""
  fi
fi

export GITHUB_TOKEN
go tool github.com/sethvargo/ratchet lint .github/workflows/*.yaml
