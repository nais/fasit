#!/usr/bin/env bash
#MISE description="Generate SQL functions and models"
#MISE depends=["fmt:prettier"]
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
