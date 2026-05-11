#!/usr/bin/env bash
#MISE description="Generate feature schema"
#MISE depends_post=["fmt:json"]
#MISE wait_for=["generate:graphql"]
#MISE sources=["internal/graph/model/*.go", "cmd/generate_schema/main.go"]
#MISE outputs=["schema/jsonschema/feature.json"]
set -euo pipefail

go run cmd/generate_schema/main.go
