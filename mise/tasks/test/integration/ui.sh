#!/usr/bin/env bash
#MISE description="Run integration tests with UI"
set -euo pipefail

go run ./cmd/tester_run --ui
