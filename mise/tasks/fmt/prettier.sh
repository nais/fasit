#!/usr/bin/env bash
#MISE description="Format SQL"
set -euo pipefail

npm ci
npx prettier --write .