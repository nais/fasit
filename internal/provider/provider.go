package provider

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/provider/protogen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	protogen.UnimplementedProviderServer
}

func newServer() protogen.ProviderServer {
	return &server{}
}

func (s *server) CreateTenant(ctx context.Context, in *protogen.CreateTenantRequest) (*protogen.TenantResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Tenant name must be at least 2 characters long")
	}

	tenant, err := environment.CreateTenant(ctx, &environment.TenantCreate{
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
	tenant, err := environment.GetTenantByName(ctx, in.Name)
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

	labels := environment.Labels{}
	for _, l := range in.Labels {
		labels[l.Key] = l.Value
	}

	env, err := environment.Create(ctx, &environment.EnvironmentCreate{
		Name:     in.Name,
		TenantID: tenant.ID,
		Kind:     kind,
		Labels:   labels,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return environmentToProto(env), nil
}

func (s *server) UpdateEnvironment(ctx context.Context, in *protogen.UpdateEnvironmentRequest) (*protogen.EnvironmentResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	environmentID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	env, err := environment.Get(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	labels := environment.Labels{}
	for _, l := range in.Labels {
		labels[l.Key] = l.Value
	}

	if err := environment.SetLabels(ctx, env.ID, labels); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if in.OidcIssuer != nil || in.OidcDiscoveryUrl != nil {
		if err := environment.SetOIDC(ctx, env.ID, in.OidcIssuer, in.OidcDiscoveryUrl); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	env, err = environment.Get(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return environmentToProto(env), nil
}

func (s *server) GetEnvironment(ctx context.Context, in *protogen.GetEnvironmentRequest) (*protogen.EnvironmentResponse, error) {
	tenantID, err := uuid.Parse(in.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid tenant id")
	}

	env, err := environment.GetByName(ctx, tenantID, in.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	return environmentToProto(env), nil
}

func (s *server) CreateOrUpdateEnvironmentValue(ctx context.Context, in *protogen.CreateOrUpdateEnvironmentValueRequest) (*protogen.CreateOrUpdateEnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	err = environment.SetEnvironmentValue(ctx, envID, in.Key, in.Value, in.Secret)
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

	ev, err := environment.GetEnvironmentValue(ctx, envID, in.Key, true)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	env, err := environment.Get(ctx, envID)
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
	es, err := environment.ListEnvironmentValuesForKey(ctx, input.GetKey())
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

	if err := environment.DeleteEnvironmentValue(ctx, uid, req.Key); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.DeleteEnvironmentValueResponse{Success: true}, nil
}

func toEnvironmentKind(kind protogen.EnvironmentKind) (environment.EnvironmentKind, error) {
	switch kind {
	case protogen.EnvironmentKind_MANAGEMENT:
		return environment.EnvironmentKindManagement, nil
	case protogen.EnvironmentKind_TENANT:
		return environment.EnvironmentKindTenant, nil
	case protogen.EnvironmentKind_ONPREM:
		return environment.EnvironmentKindOnprem, nil
	}

	return "", status.Error(codes.InvalidArgument, "Invalid Environment kind")
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func environmentToProto(env *environment.Environment) *protogen.EnvironmentResponse {
	return &protogen.EnvironmentResponse{
		Id:               env.ID.String(),
		TenantId:         env.TenantID.String(),
		Name:             env.Name,
		OidcIssuer:       derefString(env.OIDCIssuer),
		OidcDiscoveryUrl: derefString(env.OIDCDiscoveryURL),
	}
}
