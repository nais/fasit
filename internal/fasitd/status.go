package fasitd

import (
	"maps"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	recentMessagesPerSession   = 50
	recentClosedPerEnvironment = 10
)

type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

type MessageRecord struct {
	Time      time.Time
	Direction Direction
	Type      string
	Summary   string
}

type SessionRole string

const (
	RolePrimary  SessionRole = "primary"
	RoleStandby  SessionRole = "standby"
	RoleDraining SessionRole = "draining"
)

type SessionStatus struct {
	ID            uuid.UUID
	Tenant        string
	Environment   string
	Instance      string
	Version       string
	Role          SessionRole
	ConnectedAt   time.Time
	LastSeen      time.Time
	LastHeartbeat time.Time
	DrainingSince time.Time
	QueueDepth    int
	MessagesIn    map[string]int64
	MessagesOut   map[string]int64
	// Recent holds the latest messages, newest first. Heartbeats are counted
	// but not listed so they do not crowd out everything else.
	Recent []MessageRecord
}

type ClosedSession struct {
	ID          uuid.UUID
	Instance    string
	Version     string
	ConnectedAt time.Time
	ClosedAt    time.Time
	Drained     bool
	Reason      string
	MessagesIn  int64
	MessagesOut int64
}

// sessionStats is the observable state of one session, kept for the status
// page. It is safe for concurrent use and nil-safe so bare test sessions work.
type sessionStats struct {
	mu            sync.Mutex
	id            uuid.UUID
	instance      string
	connectedAt   time.Time
	lastSeen      time.Time
	lastHeartbeat time.Time
	drainingSince time.Time
	in            map[string]int64
	out           map[string]int64
	recent        []MessageRecord
	next          int
	commandNames  map[string]string
}

func newSessionStats(instance string, now time.Time) *sessionStats {
	return &sessionStats{
		id:           uuid.New(),
		instance:     instance,
		connectedAt:  now,
		lastSeen:     now,
		in:           map[string]int64{},
		out:          map[string]int64{},
		recent:       make([]MessageRecord, 0, recentMessagesPerSession),
		commandNames: map[string]string{},
	}
}

func (st *sessionStats) recordIn(typ, summary string, now time.Time) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.lastSeen = now
	st.in[typ]++
	if typ == "heartbeat" {
		st.lastHeartbeat = now
		return
	}
	st.push(MessageRecord{Time: now, Direction: DirectionIn, Type: typ, Summary: summary})
}

func (st *sessionStats) recordOut(typ, summary string, now time.Time) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.out[typ]++
	st.push(MessageRecord{Time: now, Direction: DirectionOut, Type: typ, Summary: summary})
}

func (st *sessionStats) markDraining(now time.Time) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.drainingSince.IsZero() {
		st.drainingSince = now
	}
}

// rememberCommand maps diid to a readable label so later acks, statuses and
// logs for the command can be summarised by name.
func (st *sessionStats) rememberCommand(diid, label string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.commandNames[diid] = label
}

func (st *sessionStats) commandLabel(diid string, forget bool) string {
	if st == nil {
		return diid
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	label, ok := st.commandNames[diid]
	if forget {
		delete(st.commandNames, diid)
	}
	if !ok {
		return diid
	}
	return label
}

func (st *sessionStats) push(rec MessageRecord) {
	if len(st.recent) < recentMessagesPerSession {
		st.recent = append(st.recent, rec)
		return
	}
	st.recent[st.next] = rec
	st.next = (st.next + 1) % recentMessagesPerSession
}

func (st *sessionStats) fill(out *SessionStatus) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out.ID = st.id
	out.Instance = st.instance
	out.ConnectedAt = st.connectedAt
	out.LastSeen = st.lastSeen
	out.LastHeartbeat = st.lastHeartbeat
	out.DrainingSince = st.drainingSince
	out.MessagesIn = maps.Clone(st.in)
	out.MessagesOut = maps.Clone(st.out)
	out.Recent = make([]MessageRecord, 0, len(st.recent))
	for i := range st.recent {
		idx := (st.next - 1 - i + 2*len(st.recent)) % len(st.recent)
		out.Recent = append(out.Recent, st.recent[idx])
	}
}

func (st *sessionStats) closed(version, reason string, now time.Time) ClosedSession {
	c := ClosedSession{Version: version, ClosedAt: now, Reason: reason}
	if st == nil {
		return c
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	c.ID = st.id
	c.Instance = st.instance
	c.ConnectedAt = st.connectedAt
	c.Drained = !st.drainingSince.IsZero()
	for _, n := range st.in {
		c.MessagesIn += n
	}
	for _, n := range st.out {
		c.MessagesOut += n
	}
	return c
}
