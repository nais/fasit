#!/usr/bin/env bash
#MISE description="Lint Helm charts"
set -euo pipefail

helm lint --strict ./charts/fasit
helm lint --strict ./charts/naisd