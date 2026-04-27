package breadcrumb

import (
	"github.com/nais/fasit/internal/ui"
	"github.com/nais/fasit/internal/ui/view"
)

type Crumb struct {
	Label        string
	URL          string
	Alternatives []Crumb
}

func Feature(name string) Crumb {
	return Crumb{Label: name, URL: ui.BasePath + "/features/" + name}
}

func Features() Crumb {
	return Crumb{Label: "Features", URL: ui.BasePath + "/features/"}
}

func Tenant(name string) Crumb {
	return Crumb{Label: name, URL: ui.BasePath + "/tenants/" + name}
}

func TenantWithSwitcher(name string, allTenants []view.TenantNav) Crumb {
	alts := make([]Crumb, 0, len(allTenants))
	for _, t := range allTenants {
		if t.Name != name {
			alts = append(alts, Crumb{Label: t.Name, URL: ui.BasePath + "/tenants/" + t.Name})
		}
	}
	return Crumb{Label: name, URL: ui.BasePath + "/tenants/" + name, Alternatives: alts}
}

func Environment(tenantName, envName string) Crumb {
	return Crumb{Label: envName, URL: ui.BasePath + "/tenants/" + tenantName + "/envs/" + envName}
}

func EnvironmentWithSwitcher(tenantName, envName string, environments []view.EnvironmentNav) Crumb {
	alts := make([]Crumb, 0, len(environments))
	for _, e := range environments {
		if e.Name != envName {
			alts = append(alts, Crumb{Label: e.Name, URL: ui.BasePath + "/tenants/" + tenantName + "/envs/" + e.Name})
		}
	}
	return Crumb{Label: envName, URL: ui.BasePath + "/tenants/" + tenantName + "/envs/" + envName, Alternatives: alts}
}

func EnvironmentFeature(tenantName, envName, featureName string) Crumb {
	return Crumb{Label: featureName, URL: ui.BasePath + "/tenants/" + tenantName + "/envs/" + envName + "/" + featureName}
}

func Rollouts() Crumb {
	return Crumb{Label: "Rollouts", URL: ui.BasePath + "/rollouts/"}
}

func Rollout(featureName, version string) Crumb {
	return Crumb{Label: featureName + " v" + version, URL: ui.BasePath + "/rollouts/" + featureName + "/" + version}
}

func Deployment(id, featureName, version string) Crumb {
	return Crumb{Label: featureName + " v" + version, URL: ui.BasePath + "/deployments/" + id}
}
