package graph

import (
	"encoding/json"

	"github.com/nais/fasit/pkg/feature/featureutil"
)

func pluckFromMap(key string, mp map[string]any) json.RawMessage {
	kp, _ := featureutil.SmartDotSplit(key)

	for i, k := range kp {
		v, ok := mp[k]
		if !ok {
			return nil
		}

		switch v := v.(type) {
		case map[string]any:
			if i == len(kp)-1 {
				b, _ := json.Marshal(v)
				return b
			}
			mp = v
			continue
		}

		b, _ := json.Marshal(v)
		return b
	}
	return nil
}
