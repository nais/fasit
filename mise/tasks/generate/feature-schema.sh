#!/usr/bin/env bash
#MISE description="Generate feature schema"
set -euo pipefail

go run cmd/generate_schema/main.go
