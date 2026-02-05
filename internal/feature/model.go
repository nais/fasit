package feature

import (
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
)

func makeHelmConfigMap(vals []featuresql.ConfigForEnvironmentFilteredByKeysRow) (map[string]any, error) {
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

func validateFields(f *model.Feature, envKind model.EnvironmentKind, values []featuresql.ConfigForEnvironmentFilteredByKeysRow, mp map[string]any) []string {
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
