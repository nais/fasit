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

func command(diid string) *protogen.ServerMessage {
	return &protogen.ServerMessage{Message: &protogen.ServerMessage_Command{Command: &protogen.Command{Diid: diid}}}
}

func TestRegistryOrdersPrimaryAndStandbys(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")

	first := newSession("nav", "prod")
	second := newSession("nav", "prod")
	third := newSession("nav", "prod")

	for i, s := range []*session{first, second, third} {
		if n := r.add(s); n != i+1 {
			t.Fatalf("add #%d = %d, want %d", i+1, n, i+1)
		}
	}

	if !r.isPrimary(first) || r.isPrimary(second) {
		t.Errorf("first should be primary, second should be standby")
	}

	got := r.list(key)
	if len(got) != 3 || got[0] != first || got[1] != second || got[2] != third {
		t.Fatalf("list = %v, want [first second third]", got)
	}

	r.remove(first)
	if !r.isPrimary(second) || r.isPrimary(first) {
		t.Errorf("second should be promoted to primary after removing first")
	}
	got = r.list(key)
	if len(got) != 2 || got[0] != second || got[1] != third {
		t.Fatalf("list after remove(first) = %v, want [second third]", got)
	}

	r.remove(first)
	if got := len(r.list(key)); got != 2 {
		t.Errorf("removing an already-removed session changed the list: len = %d, want 2", got)
	}
}

func TestRegistryDifferentEnvironments(t *testing.T) {
	r := newRegistry()

	r.add(newSession("nav", "prod"))
	r.add(newSession("nav", "dev"))
	if got := len(r.list(keyFor("nav", "prod"))); got != 1 {
		t.Errorf("nav/prod sessions = %d, want 1", got)
	}
	if got := len(r.list(keyFor("nav", "dev"))); got != 1 {
		t.Errorf("nav/dev sessions = %d, want 1", got)
	}
}

func TestRegistryRemoveOnlyOwnSession(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")

	first := newSession("nav", "prod")
	r.add(first)

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

func TestSessionOfferRejectsWhenFullWithoutClosing(t *testing.T) {
	s := newSession("nav", "prod")

	if !s.offer(command("a")) {
		t.Fatalf("first offer should be accepted")
	}
	if s.offer(command("b")) {
		t.Errorf("offer to a full queue should be rejected")
	}
	if !s.accepting() {
		t.Errorf("a full queue must not stop the session from accepting later commands")
	}

	<-s.send
	if !s.offer(command("c")) {
		t.Errorf("offer should be accepted again once the queue has room")
	}
}

func TestSessionStopAcceptingReturnsQueuedCommands(t *testing.T) {
	s := newSession("nav", "prod")
	s.offer(command("queued"))

	pending := s.stopAccepting()
	if len(pending) != 1 || pending[0].GetCommand().GetDiid() != "queued" {
		t.Fatalf("pending = %v, want the queued command", pending)
	}
	if s.accepting() {
		t.Errorf("session should not accept after stopAccepting")
	}
	if s.offer(command("late")) {
		t.Errorf("offer after stopAccepting should be rejected")
	}
}
