#!/usr/bin/env bash
#MISE description="Format proto files using buf"
set -euo pipefail

buf format -w schema/protobuf/provider.proto
