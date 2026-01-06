#!/usr/bin/env bash
#MISE description="Run fasit locally"
#MISE depends=["docker-compose"]
set -euo pipefail

PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-dev \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env dev \
	--log-level=debug \
	--mock-failing=true