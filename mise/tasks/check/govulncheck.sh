#!/usr/bin/env bash
#MISE description="Run govulncheck"
set -euo pipefail

go tool golang.org/x/vuln/cmd/govulncheck ./...