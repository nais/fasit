#!/usr/bin/env bash
#MISE description="Generate protobuf"
#MISE depends_post=["fmt:go"]
set -euo pipefail

mkdir -p internal/provider/protogen
protoc \
  -I schema/protobuf/ \
  ./schema/protobuf/provider.proto \
  --go_out=. \
  --go-grpc_out=.

mkdir -p internal/environmentmanagement/protogen
protoc \
  ./schema/protobuf/fasit.proto \
  --go_out=. \
  --go-grpc_out=.

mkdir -p internal/fasitd/protogen
protoc \
  -I schema/protobuf/ \
  ./schema/protobuf/fasitd.proto \
  --go_out=. \
  --go-grpc_out=.
