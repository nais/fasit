package k8s

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nais/fasit/pkg/database"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ClusterConfigMap map[string]rest.Config

func CreateClusterConfigMap(ctx context.Context, repo database.Repo) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}
	envs, err := repo.TenantEnvironments(ctx, true)
	if err != nil {
		return configs, fmt.Errorf("get all tenants environments: %w", err)
	}

	for _, env := range envs {
		configs[env.Name] = rest.Config{
			Host: fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", env, env.TenantName),
			AuthProvider: &api.AuthProviderConfig{
				Name: "google",
			},
			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return otelhttp.NewTransport(rt, otelhttp.WithServerName(env.Name))
			},
		}
	}

	return configs, nil
}

func GetClusters(ctx context.Context, repo database.Repo) ([]string, error) {
	envs, err := repo.TenantEnvironments(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("get all tenants environments: %w", err)
	}

	clusters := make([]string, len(envs))
	for i, env := range envs {
		clusters[i] = env.Name
	}

	return clusters, nil
}
