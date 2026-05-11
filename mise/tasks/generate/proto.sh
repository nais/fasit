#!/usr/bin/env bash
#MISE description="Generate protobuf"
#MISE depends_post=["fmt:go"]
#MISE sources=["schema/protobuf/provider.proto"]
#MISE outputs=["internal/provider/protogen/provider.pb.go", "internal/provider/protogen/provider_grpc.pb.go"]
set -euo pipefail

mkdir -p internal/provider/protogen
protoc \
  -I schema/protobuf/ \
  ./schema/protobuf/*.proto \
  --go_out=. \
  --go-grpc_out=.
