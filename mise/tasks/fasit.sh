#!/usr/bin/env bash
#MISE description="Run fasit locally"
#MISE depends=["docker-compose"]
set -euo pipefail

export DISABLE_COST_UPDATER=true
go run ./cmd/fasit