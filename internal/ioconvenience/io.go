package ioconvenience

import (
	"io"
	"log/slog"
)

func CloseWithLog(r io.Closer, log *slog.Logger) {
	if err := r.Close(); err != nil {
		log.With("err", err).Warn("unable to close reader")
	}
}
