#!/usr/bin/env bash
#MISE description="Build Fasit"
set -euo pipefail

go build -o ./bin/fasit ./cmd/fasit/