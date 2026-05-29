#!/usr/bin/env bash
#MISE description="Run fasit with hot reload (air)"
#MISE depends=["docker-compose"]
set -euo pipefail

cd "$(dirname "$0")/../.."

air \
  --build.cmd "go build -o ./tmp/main ./cmd/fasit" \
  --build.bin "./tmp/main" \
  --build.include_ext "go,css,js" \
  --build.exclude_dir "tmp,charts,integration_tests,schema,node_modules,.git"
