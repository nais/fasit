package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/grpc/grpcsql"
	"github.com/nais/fasit/internal/grpc/protogen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var querier grpcsql.Querier

type server struct {
	protogen.UnimplementedProviderServer
}

func NewGrpcServer(loadContext contextloader.LoaderFunc, pool *pgxpool.Pool) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(newContextInterceptor(loadContext)),
	}
	s := grpc.NewServer(opts...)
	protogen.RegisterProviderServer(s, newServer())
	querier = grpcsql.New(pool)
	return s
}

func newContextInterceptor(loadContext contextloader.LoaderFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = loadContext(ctx)
		return handler(ctx, req)
	}
}

func newServer() protogen.ProviderServer {
	return &server{}
}

func (s *server) CreateTenant(ctx context.Context, in *protogen.CreateTenantRequest) (*protogen.CreateTenantResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Tenant name must be at least 2 characters long")
	}

	tenant, err := querier.CreateTenant(ctx, grpcsql.CreateTenantParams{
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.CreateTenantResponse{
		Tenant: &protogen.Tenant{
			Name: tenant.Name,
			Id:   tenant.ID.String(),
		},
	}, nil
}

func (s *server) GetTenant(ctx context.Context, in *protogen.GetTenantRequest) (*protogen.Tenant, error) {
	tenant, err := querier.GetTenantByName(ctx, in.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	return &protogen.Tenant{
		Id:   tenant.ID.String(),
		Name: tenant.Name,
	}, nil
}

func (s *server) CreateEnvironment(ctx context.Context, in *protogen.CreateEnvironmentRequest) (*protogen.CreateEnvironmentResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Environment name must be at least 2 characters long")
	}

	tenantID, err := uuid.Parse(in.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid tenant id")
	}

	tenant, err := querier.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	kind, err := toEnvironmentKind(in.Kind)
	if err != nil {
		return nil, err
	}

	labels := types.EnvironmentLabels{}
	for _, l := range in.Labels {
		labels[l.Key] = l.Value
	}

	env, err := querier.CreateEnvironment(ctx, grpcsql.CreateEnvironmentParams{
		Name:     in.Name,
		TenantID: tenant.ID,
		Kind:     kind,
		Labels:   labels,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.CreateEnvironmentResponse{
		Environment: &protogen.Environment{
			Id:       env.ID.String(),
			TenantId: tenant.ID.String(),
			Name:     env.Name,
		},
	}, nil
}

func (s *server) SetEnvironmentLabels(ctx context.Context, in *protogen.SetEnvironmentLabelsRequest) (*protogen.SetEnvironmentLabelsResponse, error) {
	environmentID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	env, err := querier.GetEnvironment(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	labels := types.EnvironmentLabels{}
	for _, l := range in.Labels {
		labels[l.Key] = l.Value
	}

	if err := querier.SetEnvironmentLabels(ctx, grpcsql.SetEnvironmentLabelsParams{
		ID:     env.ID,
		Labels: labels,
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.SetEnvironmentLabelsResponse{
		Environment: &protogen.Environment{
			Id:       env.ID.String(),
			TenantId: env.TenantID.String(),
			Name:     env.Name,
		},
	}, nil
}

func (s *server) GetEnvironment(ctx context.Context, in *protogen.GetEnvironmentRequest) (*protogen.Environment, error) {
	tenantID, err := uuid.Parse(in.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid tenant id")
	}

	env, err := querier.GetEnvironmentByName(ctx, grpcsql.GetEnvironmentByNameParams{
		TenantID: tenantID,
		Name:     in.Name,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	return &protogen.Environment{
		Id:       env.ID.String(),
		TenantId: tenantID.String(),
		Name:     env.Name,
	}, nil
}

func (s *server) SetEnvironmentValue(ctx context.Context, in *protogen.SetEnvironmentValueRequest) (*protogen.SetEnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	err = querier.SetEnvironmentValue(ctx, grpcsql.SetEnvironmentValueParams{
		EnvironmentID: envID,
		Key:           in.Key,
		Value:         in.Value,
		Secret:        in.Secret,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &protogen.SetEnvironmentValueResponse{Success: true}, nil
}

func (s *server) GetEnvironmentValue(ctx context.Context, in *protogen.GetEnvironmentValueRequest) (*protogen.EnvironmentValue, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	ev, err := querier.GetEnvironmentValue(ctx, grpcsql.GetEnvironmentValueParams{
		EnvironmentID: envID,
		Key:           in.Key,
		ShowSensitive: true,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	env, err := querier.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	tenant, err := querier.GetTenant(ctx, env.TenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Tenant not found")
	}

	return &protogen.EnvironmentValue{
		EnvironmentId:   envID.String(),
		Key:             ev.Key,
		Value:           ev.Value,
		Secret:          ev.Secret,
		TenantId:        tenant.ID.String(),
		TenantName:      tenant.Name,
		EnvironmentName: env.Name,
	}, nil
}

func (s *server) ListEnvironmentValues(ctx context.Context, input *protogen.ListEnvironmentValuesRequest) (*protogen.ListEnvironmentValuesResponse, error) {
	es, err := environment.ListEnvironmentValuesForKey(ctx, input.GetKey())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	ret := &protogen.ListEnvironmentValuesResponse{
		Values: make([]*protogen.EnvironmentValue, len(es)),
	}

	for i, e := range es {
		ret.Values[i] = &protogen.EnvironmentValue{
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

func toEnvironmentKind(kind protogen.EnvironmentKind) (types.EnvironmentKind, error) {
	switch kind {
	case protogen.EnvironmentKind_MANAGEMENT:
		return types.EnvironmentKindManagement, nil
	case protogen.EnvironmentKind_TENANT:
		return types.EnvironmentKindTenant, nil
	case protogen.EnvironmentKind_ONPREM:
		return types.EnvironmentKindOnprem, nil
	case protogen.EnvironmentKind_LEGACY:
		return types.EnvironmentKindLegacy, nil
	}

	return "", status.Error(codes.InvalidArgument, "Invalid Environment kind")
}
