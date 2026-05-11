#!/usr/bin/env bash
#MISE description="Generate GraphQL server and models"
#MISE depends_post=["fmt:go"]
#MISE sources=["schema/*.graphqls", "gqlgen.yml"]
#MISE outputs=["internal/graph/graphgen/graphgen.go"]
set -euo pipefail

go tool github.com/99designs/gqlgen generate
