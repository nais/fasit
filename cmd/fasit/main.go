package main

import (
	"context"
	"os"

	"github.com/nais/fasit/internal/fasit"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := fasit.Run(context.Background()); err != nil {
		logrus.SetFormatter(&logrus.JSONFormatter{})
		logrus.WithError(err).Error("error occurred while running fasit")
		os.Exit(1)
	}
}
