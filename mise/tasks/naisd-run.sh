#!/usr/bin/env bash
#MISE description="Run a single naisd instance for a given tenant/env. Usage: naisd-run <tenant> <env> [--failing] [--management]"
#MISE depends=["docker-compose"]
#MISE hide=true
set -euo pipefail

if [[ $# -lt 2 ]]; then
	echo "usage: $0 <tenant> <env> [--failing] [--management]" >&2
	exit 1
fi

tenant="$1"
env="$2"
shift 2

failing=false
management=false
for arg in "$@"; do
	case "$arg" in
		--failing) failing=true ;;
		--management) management=true ;;
		*) echo "unknown arg: $arg" >&2; exit 1 ;;
	esac
done

extra=()
if [[ "$failing" == "true" ]]; then
	extra+=("--mock-failing=true")
fi
if [[ "$management" == "true" ]]; then
	extra+=("--management=true")
fi

PUBSUB_EMULATOR_HOST=localhost:8086 exec go run ./cmd/naisd \
	--env-project-id "local-${tenant}-${env}" \
	--nais-project-id nais-local-dev \
	--tenant-name "$tenant" \
	--env "$env" \
	--log-level=debug \
	"${extra[@]}"
