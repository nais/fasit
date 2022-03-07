package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/nais/fasit/pkg/status"
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := status.New[status.StatusUpdate](ctx, "banankake", "status")
	if err != nil {
		log.Fatal(err)
	}
	defer mgr.Close()

	helmStatus := status.HelmStatus{
		Name:          "loadbalancer",
		RolloutStatus: "Deployed",
		Version:       "0.1.0",
		ConfigHash:    "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
	}

	data, err := json.Marshal(helmStatus)
	if err != nil {
		log.Fatal(err)
	}

	helmStatus2 := status.HelmStatus{
		Name:          "naiserator",
		RolloutStatus: "Deployed",
		Version:       "0.1.1",
		ConfigHash:    "becce6044a0dea296c5ba0447f4d580a3ea7c847719a28a4366d1a63671b8d8e",
	}

	data2, err := json.Marshal(helmStatus2)
	if err != nil {
		log.Fatal(err)
	}

	if err := mgr.Publish(ctx, status.StatusUpdate{Partner: "mattilsynet", Environment: "dev", Type: status.StatusUpdateTypeHelm, Data: data}); err != nil {
		log.Fatal(err)
	}

	if err := mgr.Publish(ctx, status.StatusUpdate{Partner: "mattilsynet", Environment: "dev", Type: status.StatusUpdateTypeHelm, Data: data2}); err != nil {
		log.Fatal(err)
	}

	mgr.StopTopic()
}
