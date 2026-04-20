package graph

import (
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/internal/feature/featureutil"
)

// stripNoValue converts the "<no value>" sentinel produced by Go templates for
// missing map keys into nil so the playground renders YAML null values.
// Empty parent maps left behind after normalization are removed.
// This is playground-only; production rollouts are unaffected.
func stripNoValue(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			stripNoValue(val)
			if len(val) == 0 {
				delete(m, k)
			}
		case string:
			if val == "<no value>" {
				m[k] = nil
			}
		}
	}
}

func defaultsMap(valuesYAML map[string]json.RawMessage) (map[string]any, error) {
	ret := map[string]any{}

	for key, raw := range valuesYAML {
		if len(raw) == 0 {
			continue
		}

		var val any
		if err := json.Unmarshal(raw, &val); err != nil {
			return nil, err
		}

		parts, err := featureutil.SmartDotSplit(key)
		if err != nil {
			return nil, err
		}

		parent := ret
		for i, part := range parts {
			if i == len(parts)-1 {
				parent[part] = val
				break
			}

			next, ok := parent[part].(map[string]any)
			if !ok {
				if _, exists := parent[part]; exists {
					return nil, fmt.Errorf("key %q is not nestable", key)
				}
				next = map[string]any{}
				parent[part] = next
			}
			parent = next
		}
	}

	return ret, nil
}

func mergeDefaults(defaults map[string]any, resolved map[string]any) map[string]any {
	out := cloneMap(defaults)
	overlayResolved(out, resolved)
	return out
}

func overlayResolved(dst map[string]any, src map[string]any) {
	for k, v := range src {
		switch sv := v.(type) {
		case map[string]any:
			if dv, ok := dst[k].(map[string]any); ok {
				overlayResolved(dv, sv)
				continue
			}
			dst[k] = cloneMap(sv)
		case nil:
			if _, ok := dst[k]; !ok {
				dst[k] = nil
			}
		default:
			dst[k] = sv
		}
	}
}

func cloneMap(in map[string]any) map[string]any {
	ret := make(map[string]any, len(in))
	for k, v := range in {
		if vm, ok := v.(map[string]any); ok {
			ret[k] = cloneMap(vm)
			continue
		}
		ret[k] = v
	}
	return ret
}
