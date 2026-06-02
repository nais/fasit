#!/usr/bin/env bash
#MISE description="Squash database migrations"
#MISE depends=["setup"]
#MISE depends_post=["fmt:sql"]
set -euo pipefail

migrations_dir="internal/database/migrations"
files=("${migrations_dir}"/[0-9][0-9][0-9][0-9]_*.sql)
last_migration="${files[-1]##*/}"  # last file, basename
schema_file="${migrations_dir}/${last_migration%%_*}_schema.sql"  # strip everything after first _

cat > ".pgschemaignore" <<EOF
[tables]
patterns = ["goose_db_version"]
EOF

schema=$(pgschema dump --host localhost --db fasit --user postgres --password postgres --no-comments \
  | grep -v '^--')
rm -f "${migrations_dir}"/*.sql

cat > "$schema_file" <<EOF
-- +goose Up
-- +goose StatementBegin
${schema}
-- +goose StatementEnd
EOF

rm ".pgschemaignore"
echo "Squashed migrations into ${schema_file}"

