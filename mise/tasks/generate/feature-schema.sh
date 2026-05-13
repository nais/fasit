#!/usr/bin/env bash
#MISE description="Generate feature schema"
#MISE depends_post=["fmt:json"]
#MISE sources=["cmd/generate_schema/main.go"]
#MISE outputs=["schema/jsonschema/feature.json"]
set -euo pipefail

go run cmd/generate_schema/main.go
