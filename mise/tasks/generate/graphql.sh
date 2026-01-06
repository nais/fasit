#!/usr/bin/env bash
#MISE description="Generate GraphQL server and models"
set -euo pipefail

go tool github.com/99designs/gqlgen generate
