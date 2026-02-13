package provider

import (
	"context"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/provider/protogen"
	"google.golang.org/grpc"
)

func NewGrpcServer(loadContext contextloader.LoaderFunc, repo database.Repo) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(newContextInterceptor(loadContext)),
	}
	s := grpc.NewServer(opts...)
	protogen.RegisterProviderServer(s, newServer(repo))
	return s
}

func newContextInterceptor(loadContext contextloader.LoaderFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = loadContext(ctx)
		return handler(ctx, req)
	}
}
