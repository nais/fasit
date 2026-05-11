#!/usr/bin/env bash
#MISE description="Run deadcode"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

go tool golang.org/x/tools/cmd/deadcode -test ./...
