#!/usr/bin/env bash
#MISE description="Check SQL queries and schema"
#MISE sources=["internal/database/migrations/*.sql", "internal/database/queries/*.sql", "internal/*/queries/*.sql", "sqlc.yaml", "mise/config.toml"]
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc vet
