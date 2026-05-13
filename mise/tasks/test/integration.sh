#!/usr/bin/env bash
#MISE description="Run integration tests"
set -euo pipefail

go test -race -tags integration_test -cover ./...
