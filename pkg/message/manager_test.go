package message

// func TestPubSub(t *testing.T) {
// 	if err := os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085"); err != nil {
// 		t.Fatal(err)
// 	}

// 	ctx := context.Background()
// 	client, err := pubsub.NewClient(ctx, "banankake")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	mgr := NewSubscriber[Status](client, "project", "fasit-subscription")

// 	deployMgr := NewSubscriber[DeployInstruction](client, "project", "naisd-test-dev-subscription")

// 	go func() {
// 		err = mgr.Receive(ctx, func(ctx context.Context, msg Status) error {
// 			data := Helm{}
// 			err := json.Unmarshal(msg.Data, &data)
// 			if err != nil {
// 				return err
// 			}
// 			fmt.Println(data)
// 			return nil
// 		})
// 		if err != nil {
// 			t.Error(err)
// 		}
// 	}()
// 	err = deployMgr.Receive(ctx, func(ctx context.Context, msg DeployInstruction) error {
// 		fmt.Println("deploy", msg)
// 		return nil
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }
