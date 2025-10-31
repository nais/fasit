#!/usr/bin/env bash
#MISE description="Generate SQL functions and models"
#MISE depends=["check:sqlc"]
set -euo pipefail

go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
go tool mvdan.cc/gofumpt -w ./
