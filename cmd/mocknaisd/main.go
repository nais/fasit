package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/nais/fasit/pkg/status"
)

func main() {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, "banankake")
	if err != nil {
		log.Fatal(err)
	}

	mgr := status.NewPublisher[status.Update](client, "status")

	deployMgr := status.NewPublisher[status.DeployInstruction](client, "nais-test-dev")

	helmStatus := status.Helm{
		Name:          "loadbalancer",
		RolloutStatus: "Deployed",
		Version:       "0.1.0",
		ConfigHash:    "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a",
	}

	data, err := json.Marshal(helmStatus)
	if err != nil {
		log.Fatal(err)
	}

	helmStatus2 := status.Helm{
		Name:          "naiserator",
		RolloutStatus: "Deployed",
		Version:       "0.1.1",
		ConfigHash:    "becce6044a0dea296c5ba0447f4d580a3ea7c847719a28a4366d1a63671b8d8e",
	}

	data2, err := json.Marshal(helmStatus2)
	if err != nil {
		log.Fatal(err)
	}

	if err := mgr.Publish(ctx, status.Update{Partner: "test", Environment: "dev", Type: status.UpdateTypeHelm, Data: data}); err != nil {
		log.Fatal(err)
	}

	if err := mgr.Publish(ctx, status.Update{Partner: "test", Environment: "dev", Type: status.UpdateTypeHelm, Data: data2}); err != nil {
		log.Fatal(err)
	}

	deploy := status.DeployInstruction{
		Name:       "naiserator",
		Version:    "0.2.1",
		Chart:      "naiserator",
		Repo:       "",
		ConfigHash: "",
	}

	if err := deployMgr.Publish(ctx, deploy); err != nil {
		log.Fatal(err)
	}

	deployMgr.Stop()
	mgr.Stop()
}
