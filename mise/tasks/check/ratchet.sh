#!/usr/bin/env bash
#MISE description="Verify all github actions are pinned"
set -euo pipefail

if [ -z "${GITHUB_TOKEN:-}" ]; then
  if command -v gh >/dev/null 2>&1; then
    GITHUB_TOKEN="$(gh auth token)"
    export GITHUB_TOKEN
  else
    echo "Warning: GITHUB_TOKEN not set. You may hit API rate limits."
    echo "Please log in with: gh auth login"
    echo ""
  fi
fi

go tool github.com/sethvargo/ratchet lint .github/workflows/*.yaml
