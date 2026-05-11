#!/usr/bin/env bash
#MISE description="Run staticcheck"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

go tool honnef.co/go/tools/cmd/staticcheck ./...
