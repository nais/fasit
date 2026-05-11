#!/usr/bin/env bash
#MISE description="Generate spec for nais/tester"
#MISE wait_for=["generate:graphql", "generate:sqlc", "generate:mocks", "generate:proto"]
#MISE depends_post=["fmt:go", "fmt:lua"]
set -euo pipefail

# Inputs: the spec generator and the integration package (which constructs
# the test runner whose definitions become the lua spec). Plus the graph
# layer, since the runner depends on the resolver surface.
inputs=$(find cmd/tester_spec internal/integration internal/graph -name "*.go" -not -path "*/donotuse/*" -not -path "*_test.go" 2>/dev/null)
outputs="integration_tests/zz_spec.lua"

status=$(./mise/lib/stale.sh "$inputs" "$outputs")
if [ "$status" = "fresh" ]; then
	echo "tester-spec up-to-date, skipping"
	exit 0
fi

go run ./cmd/tester_spec
