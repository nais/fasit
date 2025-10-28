#!/usr/bin/env bash
#MISE description="Format go files using gofumpt"
set -euo pipefail

go tool mvdan.cc/gofumpt -w ./
