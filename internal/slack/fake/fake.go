package fake

import (
	"errors"

	"github.com/slack-go/slack"
)

type FakeSlackClient struct{}

func NewFakeSlackClient() *FakeSlackClient {
	return &FakeSlackClient{}
}

func (f *FakeSlackClient) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	return "", "", nil
}

func (f *FakeSlackClient) PostComment(channelName, messageTS string, msgOptions []slack.MsgOption) error {
	return nil
}

func (f *FakeSlackClient) UpdateMessage(channelID, timestamp string, msgOptions []slack.MsgOption) (string, string, string, error) {
	return "", "", "", errors.New("invalid_auth")
}

func (f *FakeSlackClient) AddReaction(channelID, timestamp, reaction string) error {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeProgressMessageOptions(tenant, environment, version, currentPhase, status string, startTime string, mentions string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeFailedNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeStuckNotificationMessageOptions(tenant, environment, version, status, lastModified string) []slack.MsgOption {
	return nil
}
