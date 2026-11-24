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

func TestRegistryOrdersPrimaryAndStandbys(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")

	first := newSession("nav", "prod")
	second := newSession("nav", "prod")
	third := newSession("nav", "prod")

	if n := r.add(first); n != 1 {
		t.Fatalf("add(first) = %d, want 1", n)
	}
	if n := r.add(second); n != 2 {
		t.Fatalf("add(second) = %d, want 2", n)
	}
	if n := r.add(third); n != 3 {
		t.Fatalf("add(third) = %d, want 3", n)
	}

	if !r.isPrimary(first) || r.isPrimary(second) {
		t.Errorf("first should be primary, second should be standby")
	}

	got := r.list(key)
	if len(got) != 3 || got[0] != first || got[1] != second || got[2] != third {
		t.Fatalf("list = %v, want [first second third]", got)
	}

	// Removing primary promotes second to index 0.
	r.remove(first)
	if !r.isPrimary(second) || r.isPrimary(first) {
		t.Errorf("second should be promoted to primary after removing first")
	}
	got = r.list(key)
	if len(got) != 2 || got[0] != second || got[1] != third {
		t.Fatalf("list after remove(first) = %v, want [second third]", got)
	}

	// Removing an already-removed session is a no-op.
	r.remove(first)
	if got := r.count(); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
}

func TestRegistryDifferentEnvironments(t *testing.T) {
	r := newRegistry()

	r.add(newSession("nav", "prod"))
	r.add(newSession("nav", "dev"))
	if got := r.count(); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
}

func TestRegistryRemoveOnlyOwnSession(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")

	first := newSession("nav", "prod")
	r.add(first)

	// An unregistered session with the same key must not evict the active one.
	stale := newSession("nav", "prod")
	r.remove(stale)
	if len(r.list(key)) != 1 {
		t.Errorf("active session should remain after removing an unregistered duplicate")
	}

	r.remove(first)
	if len(r.list(key)) != 0 {
		t.Errorf("session should be gone after removing it")
	}
}

func TestRegistryReconnectAfterRemove(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")

	first := newSession("nav", "prod")
	r.add(first)
	r.remove(first)

	second := newSession("nav", "prod")
	if n := r.add(second); n != 1 {
		t.Errorf("reconnecting after disconnect returned %d, want 1", n)
	}
	if !r.isPrimary(second) || len(r.list(key)) != 1 {
		t.Errorf("reconnected session should be primary")
	}
}
