package app

import "sync/atomic"

// Readiness describes lifecycle readiness, not continuous dependency health.
// The zero value is not ready. It must not be copied after first use.
// App owns the state; handlers only read it. Only successful construction may
// enable it, and all shutdown paths disable it without re-enabling it.
type Readiness struct{ ready atomic.Bool }

func (r *Readiness) SetReady(value bool) { r.ready.Store(value) }
func (r *Readiness) IsReady() bool       { return r.ready.Load() }
