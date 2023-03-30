package graph

import (
	"encoding/json"

	feature "github.com/nais/fasit/pkg/feature2"
)

func pluckFromMap(key string, mp map[string]any) json.RawMessage {
	kp, _ := feature.SmartDotSplit(key)

	for _, k := range kp {
		v, ok := mp[k]
		if !ok {
			return nil
		}

		switch v := v.(type) {
		case map[string]any:
			mp = v
		default:
			b, _ := json.Marshal(v)
			return b
		}
	}
	return nil
}
