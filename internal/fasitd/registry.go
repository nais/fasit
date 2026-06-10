package fasitd

import (
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

// registry tracks the currently connected fasitd sessions, at most one per
// environment. The session itself is the fasitd health signal, so this is
// deliberately in-memory only.
//
// TODO: only one active session per environment is supported; a second
// connection for the same tenant+environment is rejected. This needs a cleverer
// strategy (drain/handover) once fasitd moves beyond dry-run.
type registry struct {
	mu       sync.RWMutex
	sessions map[sessionKey]*session
}

func newRegistry() *registry {
	return &registry{sessions: make(map[sessionKey]*session)}
}

// add registers s unless one already exists for its environment. It returns
// false when a session is already present (the caller must reject).
func (r *registry) add(s *session) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[s.key]; ok {
		return false
	}
	r.sessions[s.key] = s
	return true
}

// remove deregisters s, but only if it is still the active session for its key.
func (r *registry) remove(s *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.sessions[s.key]; ok && cur == s {
		delete(r.sessions, s.key)
	}
}

func (r *registry) get(key sessionKey) (*session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[key]
	return s, ok
}

func (r *registry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
