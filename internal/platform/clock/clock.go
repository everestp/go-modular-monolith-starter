// Package clock provides a testable time abstraction.
package clock

import "time"

// Clock provides current time for testable code.
type Clock interface {
	Now() time.Time
}

// Real is the production clock.
type Real struct{}

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fixed is a test clock.
type Fixed struct{ now time.Time }

// NewFixed returns a fixed time for tests.
func NewFixed(now time.Time) Fixed { return Fixed{now: now.UTC()} }

// Now returns the configured fixed timestamp.
func (f Fixed) Now() time.Time { return f.now }
