package fasitd

import (
	"slices"
	"sync"

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

// session is one live fasitd connection. Outbound commands are written to send;
// the Connect handler owns the inbound direction.
type session struct {
	key           sessionKey
	environmentID uuid.UUID
	fasitdVersion string
	send          chan *protogen.ServerMessage
	done          chan struct{}
}

// registry tracks connected fasitd sessions per environment as an ordered list.
// Index 0 is the primary session; any remaining sessions are standbys tried in
// order if delivery to the primary fails or the primary drains/disconnects.
type registry struct {
	mu       sync.RWMutex
	sessions map[sessionKey][]*session
}

func newRegistry() *registry {
	return &registry{sessions: make(map[sessionKey][]*session)}
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
	list := r.sessions[key]
	if len(list) == 0 {
		return nil
	}
	out := make([]*session, len(list))
	copy(out, list)
	return out
}

// isPrimary reports whether s is currently index 0 for its environment.
func (r *registry) isPrimary(s *session) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.sessions[s.key]
	return len(list) > 0 && list[0] == s
}

func (r *registry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, list := range r.sessions {
		n += len(list)
	}
	return n
}
