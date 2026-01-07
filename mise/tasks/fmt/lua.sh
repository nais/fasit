#!/usr/bin/env bash
#MISE description="Format lua code"
set -euo pipefail

CodeFormat format -w . --ignores-file ".gitignore" -c ./integration_tests/.editorconfig