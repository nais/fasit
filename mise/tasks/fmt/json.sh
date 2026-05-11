#!/usr/bin/env bash
#MISE description="Format JSON"
set -euo pipefail

if ! command -v jq >/dev/null 2>&1; then
	echo "jq not found" >&2
	exit 1
fi

# Format each file via jq, but only rewrite the original if the content
# actually changed — preserves mtimes for downstream stale-checks.
format_one() {
	local f=$1
	local tmp
	tmp=$(mktemp)
	jq --tab "." "$f" >"$tmp"
	if ! cmp -s "$f" "$tmp"; then
		mv "$tmp" "$f"
	else
		rm -f "$tmp"
	fi
}
export -f format_one

find . -name "*.json" -type f \
	-not -path "./node_modules/*" \
	-not -path "./.vscode/*" \
	-not -path "./.git/*" \
	-print0 | xargs -0 -P 4 -I {} bash -c 'format_one "$@"' _ {}
