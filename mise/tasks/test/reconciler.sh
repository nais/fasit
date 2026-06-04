#!/usr/bin/env bash
#MISE description="Run reconciler integration tests and benchmark"
set -uo pipefail

test_exit=0
raw=$(go test -tags integration_test,reconciler_bench -v -count=1 -timeout 600s -run "TestReconcile" ./internal/reconciler/ 2>&1) || test_exit=$?

echo "$raw" | grep -E "=== RUN|--- PASS|--- FAIL|--- SKIP|took|phases|seeded|deployed|published|fetch=|FAIL"

exit $test_exit
