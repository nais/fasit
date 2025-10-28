#!/usr/bin/env bash
#MISE description="Run staticcheck"
set -euo pipefail

go tool honnef.co/go/tools/cmd/staticcheck ./...