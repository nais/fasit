#!/usr/bin/env bash
#MISE description="Generate feature schema"
#MISE depends_post=["fmt:json"]
#MISE wait_for=["generate:graphql"]
set -euo pipefail

# Inputs: the model package + the generator itself.
status=$(./mise/lib/stale.sh \
	"internal/graph/model/*.go cmd/generate_schema/main.go" \
	"schema/jsonschema/feature.json")
if [ "$status" = "fresh" ]; then
	echo "feature schema up-to-date, skipping"
	exit 0
fi

go run cmd/generate_schema/main.go
