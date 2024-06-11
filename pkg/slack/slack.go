package slack

import (
	"github.com/slack-go/slack"
)

// Slack is a client for sending messages to Slack
type Slack struct {
	client *slack.Client
}

// New creates a new Slack client
func New(token string) *Slack {
	return &Slack{
		client: slack.New(token),
	}
}

// SendMessage sends a message to a Slack channel
func (s *Slack) PostMessage(channel string, msgOptions []slack.MsgOption) error {
	_, _, err := s.client.PostMessage(channel, msgOptions...)
	return err
}
