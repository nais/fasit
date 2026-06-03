package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nais/fasit/internal/fasit"
)

func main() {
	if err := fasit.Run(context.Background()); err != nil {
		slog.With("err", err).Error("error occurred while running fasit")
		os.Exit(1)
	}
}
