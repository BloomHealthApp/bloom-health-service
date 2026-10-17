// Package clock abstracts time so services and tests can control it.
package clock

import (
	"sync/atomic"
	"time"
)

// Clock returns the current time.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// NewRealClock returns the wall clock.
func NewRealClock() Clock { return realClock{} }

// Fake is a Clock shifted by an adjustable offset from the wall clock (tests).
type Fake struct{ offset atomic.Int64 }

// NewFake returns a Fake with zero offset.
func NewFake() *Fake { return &Fake{} }

// Now returns wall time plus the offset.
func (f *Fake) Now() time.Time { return time.Now().Add(time.Duration(f.offset.Load())) }

// SetOffset shifts the clock relative to wall time.
func (f *Fake) SetOffset(d time.Duration) { f.offset.Store(int64(d)) }
