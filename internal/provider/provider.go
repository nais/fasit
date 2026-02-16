package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/provider/protogen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	protogen.UnimplementedProviderServer

	repo database.Repo
}

func newServer(repo database.Repo) protogen.ProviderServer {
	return &server{
		repo: repo,
	}
}

func (s *server) CreateTenant(ctx context.Context, in *protogen.CreateTenantRequest) (*protogen.TenantResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Tenant name must be at least 2 characters long")
	}

	tenant, err := environment.CreateTenant(ctx, &model.TenantCreate{
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

func (s *server) GetTenant(ctx context.Context, in *protogen.GetTenantRequest) (*protogen.TenantResponse, error) {
	tenant, err := environment.GetTenantGetByName(ctx, in.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	return &protogen.TenantResponse{
		Id:   tenant.ID.String(),
		Name: tenant.Name,
	}, nil
}

func (s *server) CreateEnvironment(ctx context.Context, in *protogen.CreateEnvironmentRequest) (*protogen.EnvironmentResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Environment name must be at least 2 characters long")
	}

	tenantID, err := uuid.Parse(in.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid tenant id")
	}

	tenant, err := environment.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	kind, err := toEnvironmentKind(in.Kind)
	if err != nil {
		return nil, err
	}

	env, err := s.repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     in.Name,
		TenantID: tenant.ID,
		Kind:     kind,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = s.repo.EnvironmentSetLabels(ctx, env.ID, in.Labels)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.EnvironmentResponse{
		Id:       env.ID.String(),
		TenantId: tenant.ID.String(),
		Name:     env.Name,
	}, nil
}

func (s *server) UpdateEnvironment(ctx context.Context, in *protogen.UpdateEnvironmentRequest) (*protogen.EnvironmentResponse, error) {
	environmentID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	env, err := s.repo.EnvironmentGet(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	if err := s.repo.EnvironmentSetLabels(ctx, env.ID, in.Labels); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.EnvironmentResponse{
		Id:       env.ID.String(),
		TenantId: env.TenantID.String(),
		Name:     env.Name,
	}, nil
}

func (s *server) GetEnvironment(ctx context.Context, in *protogen.GetEnvironmentRequest) (*protogen.EnvironmentResponse, error) {
	tenantID, err := uuid.Parse(in.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid tenant id")
	}

	env, err := s.repo.EnvironmentGetByName(ctx, tenantID, in.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	return &protogen.EnvironmentResponse{
		Id:       env.ID.String(),
		TenantId: tenantID.String(),
		Name:     env.Name,
	}, nil
}

func (s *server) CreateOrUpdateEnvironmentValue(ctx context.Context, in *protogen.CreateOrUpdateEnvironmentValueRequest) (*protogen.CreateOrUpdateEnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	err = s.repo.EnvironmentValueStore(ctx, envID, in.Key, in.Value, in.Secret)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &protogen.CreateOrUpdateEnvironmentValueResponse{Success: true}, nil
}

func (s *server) GetEnvironmentValue(ctx context.Context, in *protogen.GetEnvironmentValueRequest) (*protogen.EnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	ev, err := s.repo.EnvironmentValueGet(ctx, envID, in.Key, true)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	env, err := s.repo.EnvironmentGet(ctx, envID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	tenant, err := environment.GetTenant(ctx, env.TenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	return &protogen.EnvironmentValueResponse{
		EnvironmentId:   envID.String(),
		Key:             ev.Key,
		Value:           ev.Value,
		Secret:          ev.Secret,
		TenantId:        tenant.ID.String(),
		TenantName:      tenant.Name,
		EnvironmentName: env.Name,
	}, nil
}

func (s *server) GetEnvironmentValuesAcrossEnvs(ctx context.Context, input *protogen.GetEnvironmentValuesAcrossEnvsRequest) (*protogen.EnvironmentValuesAcrossEnvsResponse, error) {
	es, err := s.repo.EnvironmentValuesAcrossEnvs(ctx, input.GetKey())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	ret := &protogen.EnvironmentValuesAcrossEnvsResponse{
		Values: make([]*protogen.EnvironmentValueResponse, len(es)),
	}

	for i, e := range es {
		ret.Values[i] = &protogen.EnvironmentValueResponse{
			EnvironmentId:   e.EnvironmentID.String(),
			Key:             e.Key,
			Value:           e.Value,
			Secret:          e.Secret,
			TenantId:        e.TenantID.String(),
			TenantName:      e.TenantName,
			EnvironmentName: e.EnvironmentName,
		}
	}

	return ret, nil
}

func (s *server) DeleteEnvironmentValue(ctx context.Context, req *protogen.DeleteEnvironmentValueRequest) (*protogen.DeleteEnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	uid, err := uuid.Parse(req.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	if err := s.repo.EnvironmentValueDelete(ctx, uid, req.Key); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.DeleteEnvironmentValueResponse{Success: true}, nil
}

func toEnvironmentKind(kind protogen.EnvironmentKind) (model.EnvironmentKind, error) {
	switch kind {
	case protogen.EnvironmentKind_MANAGEMENT:
		return model.EnvironmentKindManagement, nil
	case protogen.EnvironmentKind_TENANT:
		return model.EnvironmentKindTenant, nil
	case protogen.EnvironmentKind_ONPREM:
		return model.EnvironmentKindOnprem, nil
	case protogen.EnvironmentKind_LEGACY:
		return model.EnvironmentKindLegacy, nil
	}

	return "", status.Error(codes.InvalidArgument, "Invalid Environment kind")
}
