#!/usr/bin/env bash
#MISE description="Generate mocks"
set -euo pipefail

go tool github.com/vektra/mockery/v3
