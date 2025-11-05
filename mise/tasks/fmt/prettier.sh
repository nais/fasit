#!/usr/bin/env bash
#MISE description="Format code using prettier"
set -euo pipefail

npm ci
npx prettier --write .