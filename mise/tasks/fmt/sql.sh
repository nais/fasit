#!/usr/bin/env bash
#MISE description="Format SQL"
set -euo pipefail

if ! command -v pg_format >/dev/null 2>&1; then
	echo "pg_format not found" >&2
	exit 1
fi

find . -name "*.sql" -type f \
	-not -path "./.git/*" \
	-not -path "./node_modules/*" \
	-exec pg_format --inplace --comma-break --tabs --type-case=2 --no-space-function {} +
