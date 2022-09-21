package integration_test

import (
	"time"
)

func waitFor(f func() bool) {
	for i := 0; i < 300; i++ {
		if f() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
