package naisd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

func Upgrade(ctx context.Context, mgr *DeployManager, log *logrus.Entry) error {
	log.Info("Starting upgrade")
	mgr.performNaisdUpgrades = true
	msg, err := getInstructionFromFile()
	if err != nil {
		return fmt.Errorf("naisd.Upgrade: unable to get instruction from file: %w", err)
	}

	// Use a custom context so that we don't cancel the upgrade if the service is stopped
	ctx2 := context.Background()
	return mgr.handler(ctx2, msg)
}

func getInstructionFromFile() (message.DeployInstruction, error) {
	const instructionFile = "/etc/naisd/self-upgrade/deploy_instructions.json"

	msg := message.DeployInstruction{}
	f, err := os.OpenFile(instructionFile, os.O_RDONLY, 0o644)
	if err != nil {
		return message.DeployInstruction{}, err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(&msg)
	if err != nil {
		return message.DeployInstruction{}, err
	}

	return msg, nil
}
