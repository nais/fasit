#!/usr/bin/env bash
#MISE description="Format JSON"
set -euo pipefail

find . -name "*.json" -type f | while read -r file; do
	echo "Formatting: $file"
	jq --tab '.' "$file" >"$file.tmp" && mv "$file.tmp" "$file"
done
