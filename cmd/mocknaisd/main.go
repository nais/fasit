package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/nais/fasit/pkg/status"
	"github.com/nais/fasit/pkg/workers"
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := status.New(ctx, "banankake", "status")
	if err != nil {
		log.Fatal(err)
	}
	defer mgr.Close()

	helmStatus := workers.HelmStatus{
		Name:          "naiserator",
		RolloutStatus: "Deployed",
		Version:       "6.6.6",
	}

	data, err := json.Marshal(helmStatus)
	if err != nil {
		log.Fatal(err)
	}

	if err := mgr.Publish(ctx, workers.StatusUpdate{Partner: "mattilsynet", Environment: "dev", Type: workers.StatusUpdateTypeHelm, Data: data}); err != nil {
		log.Fatal(err)
	}

	mgr.StopTopic()
}
