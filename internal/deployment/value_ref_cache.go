package deployment

import (
	"encoding/json"
)

func collectKeyRefs(deployments []*Deployment) map[string][]string {
	// key → set of feature names
	refSets := make(map[string]map[string]bool)
	for _, dep := range deployments {
		if len(dep.TplDetails) == 0 {
			continue
		}
		var details struct {
			Env        []string `json:"Env"`
			Envs       []string `json:"Envs"`
			Management []string `json:"Management"`
		}
		if err := json.Unmarshal(dep.TplDetails, &details); err != nil {
			continue
		}
		seen := make(map[string]bool)
		for _, key := range details.Env {
			seen[key] = true
		}
		for _, key := range details.Envs {
			seen[key] = true
		}
		for _, key := range details.Management {
			seen[key] = true
		}
		for key := range seen {
			if refSets[key] == nil {
				refSets[key] = make(map[string]bool)
			}
			refSets[key][dep.Feature.Name] = true
		}
	}
	refs := make(map[string][]string, len(refSets))
	for key, names := range refSets {
		for name := range names {
			refs[key] = append(refs[key], name)
		}
	}
	return refs
}
