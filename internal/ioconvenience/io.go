package ioconvenience

import (
	"io"

	"github.com/sirupsen/logrus"
)

// CloseWithLog closes the given io.Closer and logs a warning if an error occurs.
func CloseWithLog(r io.Closer, log logrus.FieldLogger) {
	if err := r.Close(); err != nil {
		log.WithError(err).Warn("unable to close reader")
	}
}
