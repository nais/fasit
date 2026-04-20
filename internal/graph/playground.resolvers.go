package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
	yaml "gopkg.in/yaml.v3"
)

// Playground is the resolver for the playground field.
func (r *mutationResolver) Playground(ctx context.Context, input model.PlaygroundInput) (*model.Playground, error) {
	retErr := func(err error) (*model.Playground, error) {
		return &model.Playground{
			Errors: []string{err.Error()},
		}, nil
	}

	env, err := r.Repo.EnvironmentByNames(ctx, input.TenantSlug, input.EnvSlug)
	if err != nil {
		return retErr(err)
	}

	fyaml := model.FeatureYAML{}
	if err := yaml.Unmarshal([]byte(input.Code), &fyaml); err != nil {
		return retErr(err)
	}

	// We have to unset required fields to make this work correctly
	for k := range fyaml.Values {
		a := fyaml.Values[k]
		a.Required = false
		fyaml.Values[k] = a
	}

	f := &model.Feature{
		FeatureYAML: fyaml,
	}
	var featureForEnv *model.Feature
	if input.FeatureName != nil {
		f.Name = *input.FeatureName
		if input.IncludeChartDefaults != nil && *input.IncludeChartDefaults {
			featureForEnv, err = featurepkg.FeatureByNameForEnv(ctx, *input.FeatureName, env.ID)
			if err != nil {
				return retErr(err)
			}
			f.Chart = featureForEnv.Chart
			f.Version = featureForEnv.Version
		}
	}
	vals, err := featurepkg.HelmValues(ctx, f, env.ID)
	if err != nil {
		return retErr(err)
	}

	stripNoValue(vals)
	if input.IncludeChartDefaults != nil && *input.IncludeChartDefaults {
		if featureForEnv == nil {
			return retErr(fmt.Errorf("featureName is required when includeChartDefaults is true"))
		}

		d, err := defaultsMap(featureForEnv.ValuesYAML)
		if err != nil {
			return retErr(err)
		}
		vals = mergeDefaults(d, vals)
	}

	if input.IncludeUnsetConfig != nil && *input.IncludeUnsetConfig {
		for k, v := range f.Values {
			if v.Config == nil || v.Computed != nil {
				continue
			}

			parts, err := featureutil.SmartDotSplit(k)
			if err != nil {
				return retErr(err)
			}

			outer := vals
			for i, part := range parts {
				if i == len(parts)-1 {
					if _, ok := outer[part]; !ok {
						outer[part] = nil
					}
					break
				}

				if _, ok := outer[part]; !ok {
					outer[part] = map[string]any{}
				}
				next, ok := outer[part].(map[string]any)
				if !ok {
					break
				}
				outer = next
			}
		}
	}

	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)

	// json.RawMessage values encode to byte arrays in YAML; normalize via JSON round-trip first.
	jsonBytes, err := json.Marshal(vals)
	if err != nil {
		return retErr(err)
	}
	var normalized any
	if err := json.Unmarshal(jsonBytes, &normalized); err != nil {
		return retErr(err)
	}

	if err := enc.Encode(normalized); err != nil {
		return retErr(err)
	}
	if err := enc.Close(); err != nil {
		return retErr(err)
	}

	s := buf.String()
	return &model.Playground{
		Result: &s,
	}, nil
}

func (r *playgroundInputResolver) IncludeChartDefaults(ctx context.Context, obj *model.PlaygroundInput, data *bool) error {
	obj.IncludeChartDefaults = data
	return nil
}

func (r *Resolver) PlaygroundInput() graphgen.PlaygroundInputResolver {
	return &playgroundInputResolver{r}
}

type playgroundInputResolver struct{ *Resolver }
