package fake

import "github.com/slack-go/slack"

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

func (f *FakeSlackClient) AddReaction(channelID, timestamp, reaction string) error {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption {
	return nil
}

func (f *FakeSlackClient) GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slack.MsgOption {
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
