package graph

import (
	"context"
	"strconv"
	"sync"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/notifier"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type logNotifier struct {
	repo database.Repo
	msgs chan *notifier.Payload

	lock        sync.RWMutex
	subscribers map[string]map[chan<- *model.LogLine]struct{}
}

func newLogNotifier(ctx context.Context, not *notifier.Notifier, repo database.Repo) *logNotifier {
	ch := not.Listen("logs")

	lf := &logNotifier{
		repo:        repo,
		subscribers: make(map[string]map[chan<- *model.LogLine]struct{}),
	}

	go lf.run(ctx, ch)

	return lf
}

func (n *logNotifier) Subscribe(diid string, ch chan<- *model.LogLine) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.subscribers[diid]; !ok {
		n.subscribers[diid] = make(map[chan<- *model.LogLine]struct{})
	}

	n.subscribers[diid][ch] = struct{}{}
}

func (n *logNotifier) Unsubscribe(diid string, ch chan<- *model.LogLine) {
	n.lock.Lock()
	defer n.lock.Unlock()

	if _, ok := n.subscribers[diid]; !ok {
		return
	}

	delete(n.subscribers[diid], ch)
}

func (n *logNotifier) run(ctx context.Context, ch <-chan notifier.Payload) {
	for msg := range ch {
		n.handleMessage(ctx, msg)
	}
}

func (n *logNotifier) handleMessage(ctx context.Context, msg notifier.Payload) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	log := logrus.WithField("data", msg.Data)
	lid, ok := msg.Data["id"]
	if !ok || lid == nil {
		log.Debug("missing id in message")
		return
	}
	lidstr, ok := lid.(string)
	if !ok {
		log.Debug("id is not a number")
		return
	}

	lidint, err := strconv.Atoi(lidstr)
	if err != nil {
		log.Debug("id is not a number")
		return
	}

	diid, ok := msg.Data["deploy_instruction"]
	if !ok || diid == nil {
		log.Debug("missing deploy_instruction in message")
		return
	}
	diidstr, ok := diid.(string)
	if !ok {
		log.Debug("deploy_instruction is not a string")
		return
	}

	subs, ok := n.subscribers[diidstr]
	if !ok || len(subs) == 0 {
		log.Debug("no subscribers")
		return
	}

	logLine, err := n.repo.LogsByID(ctx, int(lidint))
	if err != nil {
		return
	}

	for sub := range subs {
		select {
		case sub <- logLine:
		default:
		}
	}
}
