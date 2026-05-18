package breadcrumb

import "github.com/nais/fasit/internal/ui/view"

type Crumb struct {
	Label        string
	URL          string
	Alternatives []Crumb
}

func Feature(name string) Crumb {
	return Crumb{Label: name, URL: "/features/" + name}
}

func Features() Crumb {
	return Crumb{Label: "Features", URL: "/"}
}

func TenantWithSwitcher(name string, allTenants []view.TenantNav) Crumb {
	alts := make([]Crumb, 0, len(allTenants))
	for _, t := range allTenants {
		if t.Name != name {
			alts = append(alts, Crumb{Label: t.Name, URL: "/tenants/" + t.Name})
		}
	}
	return Crumb{Label: name, URL: "/tenants/" + name, Alternatives: alts}
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

func Rollouts() Crumb {
	return Crumb{Label: "Rollouts", URL: "/rollouts"}
}

func Deployments() Crumb {
	return Crumb{Label: "Deployments", URL: "/deployments"}
}

func Rollout(featureName, version string) Crumb {
	return Crumb{Label: featureName + " " + version, URL: "/rollouts/" + featureName + "/" + version}
}

func Deployment(id, featureName, target string) Crumb {
	label := featureName
	if target != "" {
		label += " — " + target
	}
	return Crumb{Label: label, URL: "/deployments/" + id}
}
