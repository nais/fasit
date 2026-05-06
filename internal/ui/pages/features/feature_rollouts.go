package features

import (
	"context"
	"strings"
	"time"

	"github.com/nais/fasit/internal/database"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RolloutItem struct {
	FeatureName string
	Version     string
	Status      string
	Created     string
	Completed   string
}

type RolloutEnvStatus struct {
	Name           string
	TenantName     string
	TenantSlug     string
	Enabled        bool
	LastModified   time.Time
	LastDeployed   time.Time
	StatusText     string
	ReleaseVersion string
}

func loadRolloutData(ctx context.Context, repo database.Repo, feature *model.Feature, data *DetailPage) {
	data.RolloutEnvs = featureEnvironmentReleaseStatuses(ctx, repo, feature)
	data.Rollouts = featureRollouts(ctx, repo, feature.Name)
}

func rolloutStatusCounts(ctx context.Context, repo database.Repo, feature *model.Feature) (failed, pending int) {
	for _, env := range featureEnvironmentReleaseStatuses(ctx, repo, feature) {
		switch strings.ToUpper(env.StatusText) {
		case "FAILED":
			failed++
		case "PENDING", "CREATED":
			pending++
		}
	}
	return failed, pending
}

func rolloutDetailContent(data *DetailPage) g.Node {
	var nodes []g.Node
	if len(data.RolloutEnvs) > 0 {
		nodes = append(nodes, envTable(data.RolloutEnvs, data.CurrentFeature.Name))
	}
	if len(data.Rollouts) > 0 {
		nodes = append(nodes, h.H2(g.Text("Rollout History")), rolloutsTable(data))
	}
	if len(nodes) == 0 {
		return h.P(g.Text("No environments or rollouts found."))
	}
	return g.Group(nodes)
}

func envTable(envs []RolloutEnvStatus, featureName string) g.Node {
	return h.Table(h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Last update")),
			h.Th(g.Text("Last deployed")),
			h.Th(g.Text("")),
		)),
		h.TBody(g.Group(g.Map(envs, func(env RolloutEnvStatus) g.Node {
			logsHref := "/tenants/" + env.TenantSlug + "/envs/" + env.Name + "/" + featureName + "/logs"
			return h.Tr(
				h.Td(g.Text(env.TenantName)),
				h.Td(h.A(h.Href("/tenants/"+env.TenantSlug+"/envs/"+env.Name+"/"+featureName), g.Text(env.Name))),
				h.Td(rolloutStatus(env.StatusText)),
				h.Td(h.Title(view.FormatTime(env.LastModified)), g.Text(view.RelativeTime(env.LastModified))),
				lastDeployedCell(env.LastDeployed),
				h.Td(h.A(h.Href(logsHref), g.Attr("title", "View logs"), g.Text("📋"))),
			)
		}))),
	)
}

func rolloutsTable(data *DetailPage) g.Node {
	return h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Version")), h.Th(g.Text("Status")), h.Th(g.Text("Created")), h.Th(g.Text("Completed")))), h.TBody(g.Group(g.Map(data.Rollouts, func(rollout RolloutItem) g.Node {
		return h.Tr(h.Td(rolloutVersionCell(rollout)), h.Td(rolloutStatus(rollout.Status)), h.Td(g.Text(rollout.Created)), h.Td(g.Text(completedDate(rollout.Completed))))
	}))))
}

func rolloutVersionCell(r RolloutItem) g.Node {
	return h.A(h.Href("/rollouts/"+r.FeatureName+"/"+r.Version), g.Text(r.Version))
}

func completedDate(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func featureRollouts(ctx context.Context, repo database.Repo, featureName string) []RolloutItem {
	rollouts, err := repo.RolloutsForFeature(ctx, featureName)
	if err != nil {
		return nil
	}
	items := make([]RolloutItem, 0, len(rollouts))
	for _, rollout := range rollouts {
		items = append(items, RolloutItem{FeatureName: rollout.FeatureName, Version: rollout.Version, Status: strings.ToUpper(rollout.Status.String()), Created: view.FormatTime(rollout.Created), Completed: view.FormatTimePtr(rollout.Completed)})
	}
	return items
}

func featureEnvironmentReleaseStatuses(ctx context.Context, repo database.Repo, feature *model.Feature) []RolloutEnvStatus {
	ret := []RolloutEnvStatus{}
	tenants, err := envpkg.GetTenants(ctx)
	if err != nil {
		return ret
	}
	for _, tenant := range tenants {
		envs, err := repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if !featureTargetsKind(feature.EnvironmentKinds, env.Kind) {
				continue
			}
			state, err := featurepkg.FeatureStateGet(ctx, env.ID, feature.Name)
			if err != nil {
				continue
			}

			es := RolloutEnvStatus{
				Name:         env.Name,
				TenantName:   tenant.Name,
				TenantSlug:   tenant.Name,
				Enabled:      state.Enabled,
				LastModified: state.LastModified,
			}

			if di, err := repo.DeployInstructionsLatestDeployedForFeature(ctx, env.ID, feature.Name); err == nil && di != nil {
				es.LastDeployed = di.LastModified
			}

			releases, err := repo.ReleaseStatusesGet(ctx, env.ID)
			if err == nil {
				for _, release := range releases {
					if release.Name == feature.Name {
						es.ReleaseVersion = release.Version
						es.StatusText = release.Status
						break
					}
				}
			}
			if es.StatusText == "" {
				if state.Enabled {
					es.StatusText = "Enabled"
				} else {
					es.StatusText = "Disabled"
				}
			}

			ret = append(ret, es)
		}
	}
	return ret
}
