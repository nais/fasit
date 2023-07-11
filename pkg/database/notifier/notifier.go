package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// Payload is the payload of a notification
// You should not change this struct, as it's used by all listeners
type Payload struct {
	Table string         `json:"table"`
	Op    string         `json:"op"`
	Data  map[string]any `json:"data"`
}

type Filter func(Payload) bool

// WithOperations returns a filter that matches the given operations
func WithOperations(ops ...string) Filter {
	return func(payload Payload) bool {
		for _, op := range ops {
			if payload.Op == op {
				return true
			}
		}

		return false
	}
}

type listener struct {
	filters []Filter
	ch      chan Payload
}

func (l *listener) match(payload Payload) bool {
	for _, filter := range l.filters {
		if !filter(payload) {
			return false
		}
	}

	return true
}

type Notifier struct {
	db      *pgxpool.Pool
	log     logrus.FieldLogger
	channel string

	lock      sync.RWMutex
	listeners map[string][]listener
}

func New(db *pgxpool.Pool, log logrus.FieldLogger) *Notifier {
	return &Notifier{
		db:        db,
		channel:   "fasit_notify",
		log:       log,
		listeners: map[string][]listener{},
	}
}

func (n *Notifier) Run(ctx context.Context) {
	retries := 0
	lastError := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := n.run(ctx); err != nil {
				n.log.WithError(err).Error("error running notifier")

				if time.Since(lastError) > time.Minute {
					retries = 0
				}

				retries++
				lastError = time.Now()

				time.Sleep(time.Duration(retries) * time.Second)
			}
		}
	}
}

// SetChannel sets the channel to listen on
// Will not take effect after Run has been called
func (n *Notifier) SetChannel(channel string) {
	n.channel = channel
}

// Listen returns a channel that will receive notifications for the given table
// It will receive all notifications for the table unless one or more filters are provided
func (n *Notifier) Listen(table string, filters ...Filter) <-chan Payload {
	n.lock.Lock()
	defer n.lock.Unlock()

	n.log.WithField("table", table).Debug("registering listener")

	ch := make(chan Payload, 5)
	n.listeners[table] = append(n.listeners[table], listener{
		filters: filters,
		ch:      ch,
	})

	return ch
}

func (n *Notifier) run(ctx context.Context) error {
	conn, err := n.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+n.channel); err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	for {
		not, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			switch {
			case pgconn.Timeout(err):
				return nil
			case errors.Is(err, io.ErrUnexpectedEOF):
				n.log.WithError(err).Infof("listener got unexpected EOF, retry")
				continue
			}

			return fmt.Errorf("wait for notification: %w", err)
		}

		payload := Payload{}
		if err := json.Unmarshal([]byte(not.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}

		go n.distibute(payload)
	}
}

func (n *Notifier) distibute(payload Payload) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	n.log.WithFields(logrus.Fields{
		"table": payload.Table,
		"op":    payload.Op,
		"data":  payload.Data,
	}).Debug("received notification")

	listeners, ok := n.listeners[payload.Table]
	if !ok {
		return
	}

	for _, listener := range listeners {
		if !listener.match(payload) {
			continue
		}

		listener.ch <- payload
	}
}
