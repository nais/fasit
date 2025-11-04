#!/usr/bin/env bash
#MISE description="Run fasit locally"
set -euo pipefail

PUBSUB_EMULATOR_HOST=localhost:8086 go run ./cmd/fasit \
	--bind-address=127.0.0.1:8080 \
	--grpc-bind-address=127.0.0.1:4444 \
	--log-level=debug \
	--insecure-skip-proxy \
	--insecure-skip-token-check