#!/usr/bin/env bash
#MISE description="Run a fasitd agent for a given tenant/env. Usage: fasitd-agent <tenant> <env>"
#MISE depends=["docker-compose"]
#MISE hide=true
set -euo pipefail

if [[ $# -lt 2 ]]; then
	echo "usage: $0 <tenant> <env>" >&2
	exit 1
fi

tenant="$1"
env="$2"

exec go run ./cmd/fasitd \
	--tenant-name "$tenant" \
	--env "$env" \
	--fasit-address localhost:4445 \
	--insecure=true \
	--log-level=debug
