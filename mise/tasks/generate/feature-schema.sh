#!/usr/bin/env bash
#MISE description="Generate feature schema"
#MISE wait_for=["generate:graphql"]
set -euo pipefail

go run cmd/generate_schema/main.go
