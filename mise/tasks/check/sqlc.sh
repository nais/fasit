#!/usr/bin/env bash
#MISE description="Check SQL queries and schema"
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc vet