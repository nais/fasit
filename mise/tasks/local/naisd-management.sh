#!/usr/bin/env bash
#MISE description="Run naisd in management environment locally"
set -euo pipefail

PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/naisd \
	--env-project-id local-test-partner-management \
	--nais-project-id nais-local-dev \
	--tenant-name test-partner \
	--env management \
	--management=true \
	--log-level=debug