#!/usr/bin/env bash
#MISE description="Run naisd for every seeded tenant/env in parallel. test-partner/prod runs --mock-failing; test-partner/staging is left unresponsive (no naisd) to produce PENDING deployments."
#MISE depends=["docker-compose"]
set -euo pipefail

root="$(cd "$(dirname "$0")" && pwd)"
runner="${root}/naisd-run.sh"

pids=()
trap 'kill "${pids[@]}" 2>/dev/null || true; wait 2>/dev/null || true' EXIT INT TERM

run() {
	local label="$1"
	shift
	(
		exec > >(while IFS= read -r line; do printf '[%s] %s\n' "$label" "$line"; done) 2>&1
		"$runner" "$@"
	) &
	pids+=("$!")
}

run "test-partner/management"          test-partner management --management
run "test-partner/dev"                 test-partner dev
run "test-partner/prod    (FAILING)"   test-partner prod        --failing
run "nav/management"                   nav          management --management
run "nav/dev"                          nav          dev
run "nav/prod"                         nav          prod
run "dev-nais/management"              dev-nais     management --management
run "dev-nais/dev"                     dev-nais     dev

wait
