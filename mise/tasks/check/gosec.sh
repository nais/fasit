#!/usr/bin/env bash
#MISE description="Run gosec"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

go tool github.com/securego/gosec/v2/cmd/gosec --exclude-generated -terse ./...
