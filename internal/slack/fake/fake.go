package fake

import (
	"github.com/slack-go/slack"
)

type FakeSlackClient struct{}

func NewFakeSlackClient() *FakeSlackClient {
	return &FakeSlackClient{}
}

func (f *FakeSlackClient) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	return "", "", nil
}

func (f *FakeSlackClient) GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption {
	return nil
}
