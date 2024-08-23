//go:build tools

package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
	_ "github.com/vektra/mockery/v2"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
