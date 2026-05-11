#!/usr/bin/env bash
#MISE description="Format YAML"
set -euo pipefail

if ! command -v yq >/dev/null 2>&1; then
	echo "yq not found" >&2
	exit 1
fi

# Format each file via yq, but only rewrite the original if the content
# actually changed — preserves mtimes so downstream stale-checks (sqlc,
# generate:*) don't re-run unnecessarily.
format_one() {
	local f=$1
	local tmp
	tmp=$(mktemp)
	yq "$f" >"$tmp"
	if ! cmp -s "$f" "$tmp"; then
		mv "$tmp" "$f"
	else
		rm -f "$tmp"
	fi
}
export -f format_one

find . -name "*.yaml" -type f \
	-not -path "./charts/*" \
	-not -path "./.git/*" \
	-not -path "./node_modules/*" \
	-print0 | xargs -0 -P 4 -I {} bash -c 'format_one "$@"' _ {}
