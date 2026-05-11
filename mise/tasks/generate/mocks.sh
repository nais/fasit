#!/usr/bin/env bash
#MISE description="Generate mocks"
#MISE wait_for=["generate:sqlc"]
#MISE depends_post=["fmt:go"]
#MISE sources=["internal/**/*.go", ".mockery.yaml"]
#MISE outputs=["internal/**/mocks/*.go"]
set -euo pipefail

go tool github.com/vektra/mockery/v3
