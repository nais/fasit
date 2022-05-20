package provider

import (
	"context"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/provider/protogen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	protogen.UnimplementedProviderServer

	repo database.Repo
}

func NewServer(repo database.Repo) protogen.ProviderServer {
	return &Server{
		repo: repo,
	}
}

func (s *Server) CreateTenant(ctx context.Context, in *protogen.CreateTenantRequest) (*protogen.TenantResponse, error) {
	if len(in.Name) < 3 {
		return nil, status.Error(codes.InvalidArgument, "Tenant name must be at least 3 characters long")
	}

	tenant, err := s.repo.TenantCreate(ctx, &model.TenantCreate{
		Name: in.Name,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.TenantResponse{
		Name: tenant.Name,
		Id:   tenant.ID.String(),
	}, nil
}
