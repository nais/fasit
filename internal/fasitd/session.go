package fasitd

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/fasitd/protogen"
)

// EnvironmentRef identifies the environment a fasitd session or command
// belongs to, by both database ID and tenant/environment name.
type EnvironmentRef struct {
	ID          uuid.UUID
	Tenant      string
	Environment string
}

func (e EnvironmentRef) key() sessionKey {
	return keyFor(e.Tenant, e.Environment)
}

// resolveEnvironment looks up an environment by tenant and environment name.
// ctx must carry the environment querier (see contextloader).
func resolveEnvironment(ctx context.Context, tenant, environment string) (EnvironmentRef, error) {
	t, err := envpkg.GetTenantByName(ctx, tenant)
	if err != nil {
		return EnvironmentRef{}, fmt.Errorf("get tenant %s: %w", tenant, err)
	}
	env, err := envpkg.GetByName(ctx, t.ID, environment)
	if err != nil {
		return EnvironmentRef{}, fmt.Errorf("get environment %s/%s: %w", tenant, environment, err)
	}
	return EnvironmentRef{ID: env.ID, Tenant: tenant, Environment: environment}, nil
}

// session is one live fasitd connection. Outbound commands are queued on send
// and written to the stream by the Connect handler.
type session struct {
	env           EnvironmentRef
	fasitdVersion string
	send          chan *protogen.ServerMessage
	done          chan struct{}
	stats         *sessionStats

	mu     sync.Mutex
	closed bool
}

func (s *session) key() sessionKey { return s.env.key() }

// offer queues msg for delivery without blocking. It returns false when the
// session no longer accepts commands (draining or disconnected) or its queue is
// full, so the caller can try the next session.
func (s *session) offer(msg *protogen.ServerMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.send <- msg:
		return true
	default:
		return false
	}
}

func (s *session) accepting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}

// stopAccepting marks the session as no longer accepting commands and returns
// any commands still queued but not yet written to the stream.
func (s *session) stopAccepting() []*protogen.ServerMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var pending []*protogen.ServerMessage
	for {
		select {
		case msg := <-s.send:
			pending = append(pending, msg)
		default:
			return pending
		}
	}
}
