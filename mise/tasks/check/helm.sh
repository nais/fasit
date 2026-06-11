#!/usr/bin/env bash
#MISE description="Lint Helm charts"
#MISE sources=["charts/**/*", "mise/config.toml"]
set -euo pipefail

helm lint --strict ./charts/fasit
helm lint --strict ./charts/naisd
helm lint --strict ./charts/fasitd
helm lint --strict ./charts/oidcproxy
