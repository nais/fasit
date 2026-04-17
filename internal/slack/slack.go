package slack

import (
	"github.com/slack-go/slack"
)

// Slack is a client for sending messages to Slack
type Slack struct {
	client *slack.Client
}

type SlackClient interface {
	PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error)
	GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption
}

// New creates a new Slack client
func New(token string) SlackClient {
	return &Slack{
		client: slack.New(token),
	}
}

// PostMessage sends a message to a Slack channel
func (s *Slack) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	channelID, timestamp, err := s.client.PostMessage(channelName, msgOptions...)
	if err != nil {
		return "", "", err
	}
	return channelID, timestamp, nil
}
