#!/usr/bin/env bash
#MISE description="Generate SQL functions and models"
#MISE depends=["fmt:sql"]
#MISE depends_post=["fmt:go"]
#MISE sources=["internal/database/migrations/*.sql", "internal/database/queries/*.sql", "internal/*/queries/*.sql", "sqlc.yaml"]
#MISE outputs=["internal/database/gensql/querier.go", "internal/*/*sql/querier.go"]
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
