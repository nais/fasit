package graph

// stripNoValue cleans up the "<no value>" sentinel produced by Go templates when
// referencing a missing map key, replacing it with nil so it renders as null in YAML.
// Empty maps (all children unresolvable) are also removed.
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
