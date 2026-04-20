package graph

// stripNoValue removes keys whose values are the "<no value>" sentinel produced by
// Go templates on missing map keys, or nil (produced by e.g. `{{ .Env.missing | quote }}`).
// Empty parent maps left behind after pruning are also removed.
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
				delete(m, k)
			}
		case nil:
			delete(m, k)
		}
	}
}
