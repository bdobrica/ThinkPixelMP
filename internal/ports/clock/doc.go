// Package clock defines the injectable domain clock boundary.
package clock

import "time"

// Clock supplies the current time. Domain and application code must use this
// port instead of reading the process clock directly.
type Clock interface{ Now() time.Time }

// System reads the process wall clock and normalizes it to UTC.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a deterministic clock intended for tests and replayable jobs.
type Fixed struct{ Time time.Time }

func (clock Fixed) Now() time.Time { return clock.Time.UTC() }
