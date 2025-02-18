package main

import (
	"context"
	"flag"
	"path/filepath"

	"github.com/nais/fasit/internal/integration"
)

func main() {
	dir := filepath.Join(".", "integration_tests")
	flag.StringVar(&dir, "d", dir, "write spec to this directory")
	flag.Parse()

	mgr, err := integration.TestRunner(context.Background(), true)
	if err != nil {
		panic(err)
	}

	if err := mgr.GenerateSpec(dir); err != nil {
		panic(err)
	}
}
