#!/usr/bin/env bash
#MISE description="Generate SQL functions and models"
#MISE depends=["fmt:sql"]
#MISE depends_post=["fmt:go"]
set -euo pipefail

# Skip if every gensql output is newer than every SQL input.
inputs="internal/database/migrations/*.sql internal/database/queries/*.sql sqlc.yaml"
for d in internal/*/queries; do
	inputs="$inputs $d/*.sql"
done
outputs="internal/database/gensql/querier.go"
for d in internal/*/*sql/querier.go; do
	outputs="$outputs $d"
done

status=$(./mise/lib/stale.sh "$inputs" "$outputs")
if [ "$status" = "fresh" ]; then
	echo "sqlc up-to-date, skipping"
	exit 0
fi

go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate
