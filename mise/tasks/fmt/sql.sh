#!/usr/bin/env bash
#MISE description="Format SQL"
set -euo pipefail

find . -name "*.sql" -exec pg_format --inplace --comma-break --tabs --type-case=2 --no-space-function {} \;
