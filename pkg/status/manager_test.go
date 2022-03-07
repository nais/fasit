package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/nais/fasit/pkg/workers"
)

func TestPubSub(t *testing.T) {
	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	mgr, err := New(ctx, "banankake", "status")
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	err = mgr.Receive(ctx, "noenytt", func(ctx context.Context, msg workers.StatusUpdate) error {
		data := workers.HelmStatus{}
		err := json.Unmarshal(msg.Data, &data)
		if err != nil {
			return err
		}
		fmt.Println(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
