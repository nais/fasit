#!/usr/bin/env bash
#MISE description="Generate tester spec"
#MISE depends_post=["fmt:lua"]
set -euo pipefail

go run ./cmd/tester_spec
