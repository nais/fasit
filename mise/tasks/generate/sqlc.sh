#!/usr/bin/env bash
#MISE description="Generate SQL functions and models"
#MISE depends=["fmt:prettier"]
#MISE depends_post=["fmt:go"]
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
