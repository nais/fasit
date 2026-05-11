#!/usr/bin/env bash
#MISE description="Generate protobuf"
#MISE depends_post=["fmt:go"]
set -euo pipefail

inputs="schema/protobuf/provider.proto"
outputs="internal/provider/protogen/provider.pb.go internal/provider/protogen/provider_grpc.pb.go"

status=$(./mise/lib/stale.sh "$inputs" "$outputs")
if [ "$status" = "fresh" ]; then
	echo "proto up-to-date, skipping"
	exit 0
fi

mkdir -p internal/provider/protogen
protoc \
  -I schema/protobuf/ \
  ./schema/protobuf/*.proto \
  --go_out=. \
  --go-grpc_out=.
