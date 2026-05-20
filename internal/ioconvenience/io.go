package ioconvenience

import (
	"io"

	"github.com/sirupsen/logrus"
)

func CloseWithLog(r io.Closer, log logrus.FieldLogger) {
	if err := r.Close(); err != nil {
		log.WithError(err).Warn("unable to close reader")
	}
}
