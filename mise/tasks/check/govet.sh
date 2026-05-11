#!/usr/bin/env bash
#MISE description="Run go vet"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

go vet ./...
