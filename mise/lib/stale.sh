#!/usr/bin/env bash
# Usage: stale.sh <inputs> <outputs>
# Each arg is a whitespace/newline-separated list of file paths.
# Prints "stale" if any input is newer than the oldest output, or any
# output is missing. Prints "fresh" otherwise.
set -euo pipefail

inputs_arg="${1:-}"
outputs_arg="${2:-}"

# shellcheck disable=SC2206
inputs=( $inputs_arg )
# shellcheck disable=SC2206
outputs=( $outputs_arg )

if [ ${#outputs[@]} -eq 0 ]; then
	echo "stale"
	exit 0
fi

# Verify every output exists.
for f in "${outputs[@]}"; do
	if [ ! -e "$f" ]; then
		echo "stale"
		exit 0
	fi
done

# Single stat invocation each — much faster than per-file fork.
# stat -f works on macOS/BSD, stat -c on Linux.
if stat -f %m / >/dev/null 2>&1; then
	stat_fmt=(-f %m)
else
	stat_fmt=(-c %Y)
fi

newest_input=$(stat "${stat_fmt[@]}" "${inputs[@]}" 2>/dev/null | sort -nr | head -1)
oldest_output=$(stat "${stat_fmt[@]}" "${outputs[@]}" 2>/dev/null | sort -n | head -1)

if [ -z "$newest_input" ] || [ -z "$oldest_output" ]; then
	echo "stale"
	exit 0
fi

if [ "$newest_input" -gt "$oldest_output" ]; then
	echo "stale"
else
	echo "fresh"
fi
