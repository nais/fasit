package stats

import "sync/atomic"

var requestCount atomic.Uint64

func IncrementRequests() {
	requestCount.Add(1)
}

func RequestCount() uint64 {
	return requestCount.Load()
}
