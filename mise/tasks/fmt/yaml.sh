#!/usr/bin/env bash
#MISE description="Format JSON"
set -euo pipefail

find . \
	-name "*.yaml" -type f -not -path "./charts/*" | while read -r file; do
	echo "Formatting: $file"
	yq --inplace "$file"
done
