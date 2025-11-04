#!/usr/bin/env bash
#MISE description="Generate mocks"
#MISE depends_post=["fmt:go"]
set -euo pipefail

go tool github.com/vektra/mockery/v3
