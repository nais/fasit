package graph

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
