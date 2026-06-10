package fasitd

import (
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/fasitd/protogen"
)

func newSession(tenant, env string) *session {
	return &session{
		key:           keyFor(tenant, env),
		environmentID: uuid.New(),
		send:          make(chan *protogen.ServerMessage, 1),
		done:          make(chan struct{}),
	}
}

func TestRegistryRejectsSecondSession(t *testing.T) {
	r := newRegistry()

	first := newSession("nav", "prod")
	if !r.add(first) {
		t.Fatalf("first session should be added")
	}

	second := newSession("nav", "prod")
	if r.add(second) {
		t.Errorf("second session for same env should be rejected")
	}

	if got := r.count(); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
}

func TestRegistryDifferentEnvironments(t *testing.T) {
	r := newRegistry()

	if !r.add(newSession("nav", "prod")) {
		t.Fatalf("nav/prod should be added")
	}
	if !r.add(newSession("nav", "dev")) {
		t.Fatalf("nav/dev should be added")
	}
	if got := r.count(); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
}

func TestRegistryRemoveOnlyOwnSession(t *testing.T) {
	r := newRegistry()

	first := newSession("nav", "prod")
	r.add(first)

	// A stale session object for the same key must not evict the active one.
	stale := newSession("nav", "prod")
	r.remove(stale)
	if _, ok := r.get(keyFor("nav", "prod")); !ok {
		t.Errorf("active session should remain after removing a stale duplicate")
	}

	r.remove(first)
	if _, ok := r.get(keyFor("nav", "prod")); ok {
		t.Errorf("session should be gone after removing the active one")
	}
}

func TestRegistryReconnectAfterRemove(t *testing.T) {
	r := newRegistry()

	first := newSession("nav", "prod")
	r.add(first)
	r.remove(first)

	second := newSession("nav", "prod")
	if !r.add(second) {
		t.Errorf("reconnecting after disconnect should be allowed")
	}
}
