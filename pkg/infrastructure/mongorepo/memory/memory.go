// Package memory provides in-memory fakes of the mongorepo repositories. They follow the same
// semantics (typed errors, CAS-on-version, per-space seq, unique constraints) and back tests and
// the STORE=memory development mode.
package memory

import (
	"context"
)

// Pinger is an always-healthy drivers.IPinger.
type Pinger struct{}

// Ping always succeeds.
func (Pinger) Ping(context.Context) error { return nil }
