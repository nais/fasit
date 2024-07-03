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
func (s *Slack) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	channelId, timestamp, err := s.client.PostMessage(channelName, msgOptions...)
	if err != nil {
		return "", "", err
	}
	return channelId, timestamp, nil
}

func (s *Slack) PostComment(channelName, messageTs string, msgOptions []slack.MsgOption) error {
	msgOptions = append(msgOptions, slack.MsgOptionTS(messageTs))
	_, _, err := s.client.PostMessage(channelName, msgOptions...)
	return err
}

func (s *Slack) AddReaction(channelId, timestamp, reaction string) error {
	return s.client.AddReaction(reaction, slack.ItemRef{Channel: channelId, Timestamp: timestamp})
}
