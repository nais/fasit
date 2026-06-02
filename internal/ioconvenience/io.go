package ioconvenience

import (
	"io"
	"log/slog"
)

func CloseWithLog(r io.Closer, log *slog.Logger) {
	if err := r.Close(); err != nil {
		log.Warn("unable to close reader", "error", err)
	}
}
