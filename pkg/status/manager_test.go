package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestPubSub(t *testing.T) {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := New[Update](ctx, "banankake", "status")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	deployMgr, err := New[DeployInstruction](ctx, "banankake", "naisd-mattilsynet-dev")
	if err != nil {
		t.Fatal(err)
	}
	defer deployMgr.Close()

	go func() {
		err = mgr.Receive(ctx, "status", func(ctx context.Context, msg Update) error {
			data := Helm{}
			err := json.Unmarshal(msg.Data, &data)
			if err != nil {
				return err
			}
			fmt.Println(data)
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	}()
	err = deployMgr.Receive(ctx, "deploysss", func(ctx context.Context, msg DeployInstruction) error {
		fmt.Println("deploy", msg)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
