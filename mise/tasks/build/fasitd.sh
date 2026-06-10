#!/usr/bin/env bash
#MISE description="Build fasitd"
set -euo pipefail

go build -o ./bin/fasitd ./cmd/fasitd/
