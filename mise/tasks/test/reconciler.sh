#!/usr/bin/env bash
#MISE description="Run reconciler integration tests and compare old vs new timing"
set -uo pipefail

test_exit=0
raw=$(go test -tags integration_test,reconciler_bench -v -count=1 -timeout 600s -run "TestReconcile" ./internal/deployment/ 2>&1) || test_exit=$?

echo "$raw" | grep -E "=== RUN|--- PASS|--- FAIL|took|phases|seeded|deployed" | sed 's/^    //'

echo ""
echo "=== Timing comparison ==="
printf "%-55s %14s %14s %8s\n" "Test" "Old" "New" "Speedup"
printf "%-55s %14s %14s %8s\n" "----" "---" "---" "-------"

# Parse lines into old_file / new_file keyed by "parent #N"
old_file=$(mktemp)
new_file=$(mktemp)
phases_file=$(mktemp)
trap 'rm -f "$old_file" "$new_file" "$phases_file"' EXIT

current_test=""
while IFS= read -r line; do
    if [[ "$line" == *"=== RUN"* ]]; then
        current_test="${line#*=== RUN   }"
        current_test="${current_test#"${current_test%%[![:space:]]*}"}"
    fi
    if [[ "$line" == *"took "* ]]; then
        duration="${line##*took }"
        num="${line##*reconcile #}"
        num="${num%% *}"
        if [[ "$current_test" == *"/old"* ]]; then
            parent="${current_test%/old*}"
            echo "${parent} #${num}	${duration}" >> "$old_file"
        elif [[ "$current_test" == *"/new"* ]]; then
            parent="${current_test%/new*}"
            echo "${parent} #${num}	${duration}" >> "$new_file"
        fi
    fi
    if [[ "$line" == *"phases:"* ]]; then
        phases="${line##*phases: }"
        if [[ "$current_test" == *"/new"* ]]; then
            parent="${current_test%/new*}"
            # Get the last reconcile number from new_file
            last_num=$(tail -1 "$new_file" | cut -d'#' -f2 | cut -f1)
            echo "${parent} #${last_num}	${phases}" >> "$phases_file"
        fi
    fi
done <<< "$raw"

to_ms() {
    local val="$1"
    if [[ "$val" == *"µs" ]]; then
        awk "BEGIN{printf \"%.3f\", ${val%µs}/1000}"
    elif [[ "$val" == *"ms" ]]; then
        echo "${val%ms}"
    elif [[ "$val" == *"s" ]]; then
        awk "BEGIN{printf \"%.3f\", ${val%s}*1000}"
    else
        echo ""
    fi
}

while IFS=$'\t' read -r key old_val; do
    new_val=$(grep -F "$key" "$new_file" 2>/dev/null | cut -f2 || echo "n/a")
    new_val="${new_val:-n/a}"
    phases=$(grep -F "$key" "$phases_file" 2>/dev/null | cut -f2 || echo "")

    speedup="n/a"
    old_ms=$(to_ms "$old_val")
    new_ms=$(to_ms "$new_val")
    if [[ -n "$old_ms" ]] && [[ -n "$new_ms" ]]; then
        speedup=$(awk "BEGIN { printf \"%.1fx\", $old_ms / $new_ms }")
    fi

    printf "%-55s %14s %14s %8s\n" "$key" "$old_val" "$new_val" "$speedup"
    if [[ -n "$phases" ]]; then
        printf "%-55s %s\n" "" "  └─ $phases"
    fi
done < "$old_file"

exit $test_exit
