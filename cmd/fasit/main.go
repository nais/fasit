package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nais/fasit/internal/fasit"
)

func main() {
	if err := fasit.Run(context.Background()); err != nil {
		slog.Error("error occurred while running fasit", "error", err)
		os.Exit(1)
	}
}
