#!/usr/bin/env bash
#MISE description="Run fasit locally"
#MISE depends=["docker-compose"]
set -euo pipefail

export FASIT_FAKE_NAISD=true
go run ./cmd/fasit