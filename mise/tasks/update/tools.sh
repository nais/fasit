#!/usr/bin/env bash
#MISE description="Update all Go tools to latest"
set -euo pipefail

tools=()
while IFS= read -r tool; do
  tools+=("${tool}@latest")
done < <(go list tool)

if [ "${#tools[@]}" -eq 0 ]; then
  echo "No tools found in go.mod" >&2
  exit 1
fi

go get -tool "${tools[@]}"
go mod tidy
