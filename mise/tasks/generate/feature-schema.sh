#!/usr/bin/env bash
#MISE description="Generate feature schema"
#MISE depends_post=["fmt:prettier"]
set -euo pipefail

go run cmd/generate_schema/main.go
