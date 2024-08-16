//go:build tools

package tools

import (
	_ "github.com/99designs/gqlgen"
	_ "github.com/sqlc-dev/sqlc/cmd/sqlc"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
