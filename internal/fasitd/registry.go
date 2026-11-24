package fasitd

import (
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/fasitd/protogen"
)

// sessionKey identifies the environment a session belongs to.
type sessionKey struct {
	tenant      string
	environment string
}

func keyFor(tenant, environment string) sessionKey {
	return sessionKey{tenant: tenant, environment: environment}
}

// session is one live fasitd connection. Outbound commands are queued on send
// and written to the stream by the Connect handler.
type session struct {
	key           sessionKey
	environmentID uuid.UUID
	fasitdVersion string
	send          chan *protogen.ServerMessage
	done          chan struct{}
	stats         *sessionStats

	mu     sync.Mutex
	closed bool
}

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

// registry tracks connected fasitd sessions per environment as an ordered list.
// Index 0 is the primary session; any remaining sessions are standbys tried in
// order if the primary does not accept a command or drains/disconnects.
//
// Separately from routing it tracks every connected session (including
// draining ones) and the most recently closed sessions, for the status page.
type registry struct {
	mu               sync.RWMutex
	sessions         map[sessionKey][]*session
	connected        map[sessionKey][]*session
	closed           map[sessionKey][]ClosedSession
	releasesReported map[uuid.UUID]time.Time
}

func newRegistry() *registry {
	return &registry{
		sessions:         make(map[sessionKey][]*session),
		connected:        make(map[sessionKey][]*session),
		closed:           make(map[sessionKey][]ClosedSession),
		releasesReported: make(map[uuid.UUID]time.Time),
	}
}

func (r *registry) recordReleasesReport(envID uuid.UUID, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releasesReported[envID] = at
}

func (r *registry) releasesReportedAt(envID uuid.UUID) time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.releasesReported[envID]
}

func (r *registry) track(s *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connected[s.key] = append(r.connected[s.key], s)
}

// untrack records s as closed with reason, keeping the most recent closures
// per environment newest first.
func (r *registry) untrack(s *session, reason string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if list := slices.DeleteFunc(r.connected[s.key], func(c *session) bool { return c == s }); len(list) > 0 {
		r.connected[s.key] = list
	} else {
		delete(r.connected, s.key)
	}
	closed := append([]ClosedSession{s.stats.closed(s.fasitdVersion, reason, now)}, r.closed[s.key]...)
	if len(closed) > recentClosedPerEnvironment {
		closed = closed[:recentClosedPerEnvironment]
	}
	r.closed[s.key] = closed
}

// status returns the live sessions (routing order first, then draining) and
// recent closures for key.
func (r *registry) status(key sessionKey) ([]SessionStatus, []ClosedSession) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routing := r.sessions[key]
	live := make([]SessionStatus, 0, len(r.connected[key]))
	for _, s := range r.connected[key] {
		st := SessionStatus{
			Tenant:      key.tenant,
			Environment: key.environment,
			Version:     s.fasitdVersion,
			QueueDepth:  len(s.send),
		}
		switch idx := slices.Index(routing, s); {
		case idx == 0:
			st.Role = RolePrimary
		case idx > 0:
			st.Role = RoleStandby
		default:
			st.Role = RoleDraining
		}
		s.stats.fill(&st)
		live = append(live, st)
	}
	slices.SortStableFunc(live, func(a, b SessionStatus) int {
		return rolePriority(a, routing) - rolePriority(b, routing)
	})
	return live, slices.Clone(r.closed[key])
}

func rolePriority(st SessionStatus, routing []*session) int {
	for i, s := range routing {
		if s.stats != nil && s.stats.id == st.ID {
			return i
		}
	}
	return len(routing)
}

// add appends s to the end of its environment's session list and returns the
// new number of sessions for that environment.
func (r *registry) add(s *session) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.key] = append(r.sessions[s.key], s)
	return len(r.sessions[s.key])
}

// remove deletes s from its environment's session list. Removing the primary
// promotes the next session to index 0.
func (r *registry) remove(s *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.sessions[s.key]
	idx := slices.Index(list, s)
	if idx < 0 {
		return
	}
	list = slices.Delete(list, idx, idx+1)
	if len(list) == 0 {
		delete(r.sessions, s.key)
		return
	}
	r.sessions[s.key] = list
}

// list returns a snapshot of sessions for key in priority order (index 0 first).
func (r *registry) list(key sessionKey) []*session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.sessions[key])
}

func (r *registry) isPrimary(s *session) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.sessions[s.key]
	return len(list) > 0 && list[0] == s
}
