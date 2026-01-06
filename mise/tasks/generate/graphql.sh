#!/usr/bin/env bash
#MISE description="Generate GraphQL server and models"
#MISE depends_post=["fmt:go"]
set -euo pipefail

go tool github.com/99designs/gqlgen generate
