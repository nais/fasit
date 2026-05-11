#!/usr/bin/env bash
#MISE description="Run govulncheck"
#MISE sources=["**/*.go", "go.mod", "go.sum", "mise/config.toml"]
set -euo pipefail

go tool golang.org/x/vuln/cmd/govulncheck ./...
