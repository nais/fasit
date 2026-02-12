package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
)

type ConfigRepo interface {
	HelmValues(ctx context.Context, feature *model.Feature, envID uuid.UUID) (values map[string]any, err error)
}

func (r *repo) HelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	mv, envKind, err := r.MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, f := range f.Values {
		if f.Config != nil && !contains(f.IgnoreKind, envKind) {
			includeKeys = append(includeKeys, key)
		}
	}

	vals, err := r.querier.EnvConfigOnlyKnown(ctx, gensql.EnvConfigOnlyKnownParams{
		Feature:       f.Name,
		EnvironmentID: envID,
		Includedkeys:  includeKeys,
	})
	if err != nil {
		return nil, err
	}

	mp, err := makeHelmConfigMap(vals)
	if err != nil {
		return nil, err
	}

	err = feature.Generate(f.Values, envKind, mv, mp)

	mp["fasit"] = map[string]any{
		"tenant": map[string]string{
			"name": mv.Tenant.Name,
		},
		"env": map[string]string{
			"name": mv.Env["name"].(string),
			"kind": envKind.String(),
		},
	}

	missing := validateFields(f, envKind, vals, mp)
	if len(missing) > 0 {
		return nil, &errs.ErrMissingRequiredFields{Fields: missing}
	}

	return mp, err
}

func validateFields(f *model.Feature, envKind model.EnvironmentKind, values []gensql.EnvConfigOnlyKnownRow, mp map[string]any) []string {
	requiredFields := f.RequiredFields(envKind)

	fields := map[string]bool{}
	for _, req := range requiredFields {
		fields[req] = false
		for _, k := range values {
			if k.Key == req {
				fields[req] = true
			}
		}
	}

	var missing []string
	for field, present := range fields {
		if present {
			continue
		}

		parts, _ := featureutil.SmartDotSplit(field)
		parent := mp
		for _, part := range parts {
			if p, ok := parent[part].(map[string]any); ok {
				parent = p
				continue
			}
			if _, ok := parent[part]; ok {
				continue
			}
			missing = append(missing, field)
			break
		}
	}
	return missing
}

func makeHelmConfigMap(vals []gensql.EnvConfigOnlyKnownRow) (map[string]any, error) {
	val := make(map[string]any)

	for _, v := range vals {
		keys, err := featureutil.SmartDotSplit(v.Key)
		if err != nil {
			return nil, err
		}
		parent := val
		for index, key := range keys {
			if index == len(keys)-1 {
				parent[key] = json.RawMessage(v.Value)
				continue
			}
			if e, ok := parent[key]; ok {
				if p, ok := e.(map[string]any); ok {
					if index == len(keys)-1 {
						return nil, fmt.Errorf("key %v is not nestable", v.Key)
					}
					parent = p
					continue
				}
				return nil, fmt.Errorf("key %v is not nestable", v.Key)
			}
			f := make(map[string]any)
			parent[key] = f
			parent = f
		}
	}
	return val, nil
}

func contains[T comparable](s []T, e T) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
