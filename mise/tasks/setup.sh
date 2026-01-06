#!/usr/bin/env bash
#MISE description="Setup local environment"
#MISE depends=["docker-compose"]
set -euo pipefail

go run cmd/setup_local_env/main.go