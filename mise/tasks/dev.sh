#!/usr/bin/env bash
#MISE description="Run fasit with hot reload (air)"
#MISE depends=["docker-compose"]
set -euo pipefail

cd "$(dirname "$0")/../.."

export FASIT_FAKE_NAISD=true
# Seed a couple of orphaned Helm releases (installed, but no matching feature
# assignment) so the environment Helm Releases tab shows the orphan + uninstall
# flow locally. Format: tenant/env/release.
export FASIT_FAKE_NAISD_ORPHAN_RELEASES="nav/dev/legacy-logging,test-partner/prod/old-proxy"
air \
  --build.cmd "go build -o ./tmp/main ./cmd/fasit" \
  --build.bin "./tmp/main" \
  --build.include_ext "go,css,js" \
  --build.exclude_dir "tmp,charts,integration_tests,schema,node_modules,.git"
