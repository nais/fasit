#!/usr/bin/env bash
#MISE description="Run go vet"
set -euo pipefail

go vet ./...
