#!/usr/bin/env bash
#MISE description="Build naisd"
set -euo pipefail

go build -o ./bin/naisd ./cmd/naisd/