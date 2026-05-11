#!/usr/bin/env bash
#MISE description="Generate mocks"
#MISE wait_for=["generate:sqlc"]
#MISE depends_post=["fmt:go"]
set -euo pipefail

# Stale-check: mocks need regen if any non-mock .go file or .mockery.yaml
# is newer than the oldest existing mock file.
inputs=$(find internal -name "*.go" -not -path "*/mocks/*" -not -path "*/donotuse/*" -not -path "*_test.go" 2>/dev/null)
inputs="$inputs .mockery.yaml"
outputs=$(find internal -path "*/mocks/*.go" 2>/dev/null)

if [ -n "$outputs" ]; then
	status=$(./mise/lib/stale.sh "$inputs" "$outputs")
	if [ "$status" = "fresh" ]; then
		echo "mocks up-to-date, skipping"
		exit 0
	fi
fi

go tool github.com/vektra/mockery/v3
