package artistart

import (
	"sync"
	"sync/atomic"
)

// ActivityGate solves a coordination problem grilling round Q1 raised:
// watchlist.Service.Add's add-time match and artistart.Backfill's startup
// sweep (sibling plan 13-03) share the same rate-limited dzClient/mbClient
// instances, and nothing coordinates them -- a startup sweep with a
// nonzero backlog can make every interactive add compete for the same
// limiter right after every deploy.
//
// ActivityGate is not a second rate budget -- that was considered and
// rejected (see D-10's note on Q1(b)): MusicBrainz's external ~1 req/sec
// ceiling is a whole-service constraint, and a second independent limiter
// would just let the backfill exceed it rather than share it. This is a
// plain "is an interactive request in flight" signal the backfill can
// check and yield to.
type ActivityGate struct {
	count atomic.Int32
}

// NewActivityGate returns a ready-to-use zero-count gate.
func NewActivityGate() *ActivityGate {
	return &ActivityGate{}
}

// Begin marks one interactive request as starting and returns an end func
// that marks it as finished. Callers must invoke end exactly once, normally
// via defer -- but the returned closure is guarded by sync.Once, so a
// caller that accidentally invokes it twice cannot decrement the counter
// twice and drive it negative, which would make Active permanently or
// intermittently wrong.
func (g *ActivityGate) Begin() (end func()) {
	g.count.Add(1)
	var once sync.Once
	return func() {
		once.Do(func() {
			g.count.Add(-1)
		})
	}
}

// Active reports whether at least one Begin() call has not yet had its
// matching end() invoked.
func (g *ActivityGate) Active() bool {
	return g.count.Load() > 0
}
