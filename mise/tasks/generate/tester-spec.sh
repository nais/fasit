#!/usr/bin/env bash
#MISE description="Generate spec for nais/tester"
#MISE wait_for=["generate:graphql", "generate:sqlc", "generate:mocks", "generate:proto"]
#MISE depends_post=["fmt:go", "fmt:lua"]
set -euo pipefail

go run ./cmd/tester_spec