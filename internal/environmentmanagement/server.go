package environmentmanagement

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environmentmanagement/protogen"
	"github.com/nais/fasit/internal/environmentmanagement/sqlgen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	protogen.UnimplementedFasitServer
	querier sqlgen.Querier
}

func NewGrpcServer(loadContext contextloader.LoaderFunc, pool *pgxpool.Pool) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(newContextInterceptor(loadContext)),
	}
	s := grpc.NewServer(opts...)
	protogen.RegisterFasitServer(s, &server{querier: sqlgen.New(pool)})
	return s
}

func newContextInterceptor(loadContext contextloader.LoaderFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = loadContext(ctx)
		return handler(ctx, req)
	}
}

func (s *server) CreateTenant(ctx context.Context, in *protogen.CreateTenantRequest) (*protogen.CreateTenantResponse, error) {
	// TODO: remember to add audit logs, in this func and other funcs in the server

	ctx = auth.SetEmail(ctx, "system:provider")

	if len(in.Name) < 2 {
		return nil, status.Error(codes.InvalidArgument, "Tenant name must be at least 2 characters long")
	}

	tenant, err := s.querier.CreateTenant(ctx, sqlgen.CreateTenantParams{
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
	tenant, err := s.querier.GetTenantByName(ctx, in.Name)
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

	tenant, err := s.querier.GetTenant(ctx, tenantID)
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

	env, err := s.querier.CreateEnvironment(ctx, sqlgen.CreateEnvironmentParams{
		Name:     in.Name,
		TenantID: tenant.ID,
		Kind:     kind,
		Labels:   labels,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.CreateEnvironmentResponse{
		Environment: environmentToProto(env),
	}, nil
}

func (s *server) SetEnvironmentLabels(ctx context.Context, in *protogen.SetEnvironmentLabelsRequest) (*protogen.SetEnvironmentLabelsResponse, error) {
	environmentID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	env, err := s.querier.GetEnvironment(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	labels := types.EnvironmentLabels{}
	for _, l := range in.Labels {
		labels[l.Key] = l.Value
	}

	if err := s.querier.SetEnvironmentLabels(ctx, sqlgen.SetEnvironmentLabelsParams{
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

	env, err := s.querier.GetEnvironmentByName(ctx, sqlgen.GetEnvironmentByNameParams{
		TenantID: tenantID,
		Name:     in.Name,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	return environmentToProto(env), nil
}

func (s *server) SetEnvironmentOIDC(ctx context.Context, in *protogen.SetEnvironmentOIDCRequest) (*protogen.SetEnvironmentOIDCResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	environmentID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	if _, err := s.querier.GetEnvironment(ctx, environmentID); err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	issuer := optionalString(in.OidcIssuer)
	discovery := optionalString(in.OidcDiscoveryUrl)
	if err := s.querier.SetEnvironmentOIDC(ctx, sqlgen.SetEnvironmentOIDCParams{
		ID:               environmentID,
		OidcIssuer:       issuer,
		OidcDiscoveryUrl: discovery,
	}); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	env, err := s.querier.GetEnvironment(ctx, environmentID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &protogen.SetEnvironmentOIDCResponse{Environment: environmentToProto(env)}, nil
}

func (s *server) SetEnvironmentValue(ctx context.Context, in *protogen.SetEnvironmentValueRequest) (*protogen.SetEnvironmentValueResponse, error) {
	ctx = auth.SetEmail(ctx, "system:provider")

	envID, err := uuid.Parse(in.EnvironmentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "Invalid environment id")
	}

	err = s.querier.SetEnvironmentValue(ctx, sqlgen.SetEnvironmentValueParams{
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

	ev, err := s.querier.GetEnvironmentValue(ctx, sqlgen.GetEnvironmentValueParams{
		EnvironmentID: envID,
		Key:           in.Key,
		ShowSensitive: true,
	})
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	env, err := s.querier.GetEnvironment(ctx, envID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "Environment not found")
	}

	tenant, err := s.querier.GetTenant(ctx, env.TenantID)
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
	es, err := s.querier.ListEnvironmentValuesForKey(ctx, input.GetKey())
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

	if err := s.querier.DeleteEnvironmentValue(ctx, sqlgen.DeleteEnvironmentValueParams{EnvironmentID: uid, Key: req.Key}); err != nil {
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
	}

	return "", status.Error(codes.InvalidArgument, "Invalid Environment kind")
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func environmentToProto(env sqlgen.Environment) *protogen.Environment {
	return &protogen.Environment{
		Id:               env.ID.String(),
		TenantId:         env.TenantID.String(),
		Name:             env.Name,
		OidcIssuer:       derefString(env.OidcIssuer),
		OidcDiscoveryUrl: derefString(env.OidcDiscoveryUrl),
	}
}
