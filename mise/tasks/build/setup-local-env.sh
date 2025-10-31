#!/usr/bin/env bash
#MISE description="Build setup_local_env"
set -euo pipefail

go build -o ./bin/setup_local_env ./cmd/setup_local_env/