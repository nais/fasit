#!/usr/bin/env bash
#MISE description="Generate protobuf"
set -euo pipefail

mkdir -p internal/provider/protogen
protoc \
  -I schema/protobuf/ \
  ./schema/protobuf/*.proto \
  --go_out=. \
  --go-grpc_out=.
