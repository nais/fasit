#!/usr/bin/env bash
#MISE description="Run all tests"
set -euo pipefail

go test -race -tags integration_test -cover ./...