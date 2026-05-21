package breadcrumb

import (
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
)

type Crumb struct {
	Label        string
	URL          string
	Subtitle     string // shown in parentheses after Label on the last crumb
	SourceURL    string // shown as icon link to the right on the last crumb
	Content      g.Node // if set, rendered instead of Label for last crumb
	Alternatives []Crumb
}

func Feature(name string) Crumb {
	return Crumb{Label: name, URL: "/features/" + name}
}

func Features() Crumb {
	return Crumb{Label: "Features", URL: "/features"}
}

func TenantWithSwitcher(name string, allTenants []view.TenantNav) Crumb {
	return Crumb{Label: name}
}

func EnvironmentWithSwitcher(tenantName, envName string, environments []view.EnvironmentNav) Crumb {
	alts := make([]Crumb, 0, len(environments))
	for _, e := range environments {
		if e.Name != envName {
			alts = append(alts, Crumb{Label: e.Name, URL: "/tenants/" + tenantName + "/envs/" + e.Name})
		}
	}
	return Crumb{Label: envName, URL: "/tenants/" + tenantName + "/envs/" + envName, Alternatives: alts}
}

func EnvironmentFeature(tenantName, envName, featureName string) Crumb {
	return Crumb{Label: featureName, URL: "/tenants/" + tenantName + "/envs/" + envName + "/" + featureName}
}

func Deployments() Crumb {
	return Crumb{Label: "Deployments", URL: "/deployments"}
}

func Deployment(id, featureName, version string) Crumb {
	return Crumb{Label: featureName + " " + version, URL: "/deployments/" + id}
}
