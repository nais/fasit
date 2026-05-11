#!/usr/bin/env bash
#MISE description="Generate GraphQL server and models"
#MISE depends_post=["fmt:go"]
set -euo pipefail

# Skip if generated output is newer than every schema file + config.
status=$(./mise/lib/stale.sh \
	"schema/*.graphqls gqlgen.yml" \
	"internal/graph/graphgen/graphgen.go")
if [ "$status" = "fresh" ]; then
	echo "graphql up-to-date, skipping"
	exit 0
fi

go tool github.com/99designs/gqlgen generate
