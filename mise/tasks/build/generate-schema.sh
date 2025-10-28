#!/usr/bin/env bash
#MISE description="Build generate_schema"
set -euo pipefail

go build -o ./bin/generate_schema ./cmd/generate_schema/