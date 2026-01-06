#!/usr/bin/env bash
#MISE description="Run integration tests"
set -euo pipefail

go run ./cmd/tester_run
go test -race -tags integration_test -cover ./...
