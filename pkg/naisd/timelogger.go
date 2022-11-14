package naisd

import (
	"encoding/json"
	"time"
)

type timeLogger struct {
	lines []map[string]any
}

func (t *timeLogger) Write(p []byte) (n int, err error) {
	t.lines = append(t.lines, map[string]any{
		"time": time.Now(),
		"msg":  string(p),
	})

	return len(p), nil
}

func (t *timeLogger) String() string {
	b, _ := json.Marshal(t.lines)
	return string(b)
}
