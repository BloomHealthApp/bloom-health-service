// Package friendrelation enumerates how the caller relates to a profile opened from a link.
package friendrelation

const (
	Self     = "self"     // the caller's own link
	None     = "none"     // no request either way
	Outgoing = "outgoing" // the caller asked and is waiting
	Incoming = "incoming" // they asked the caller
	Friends  = "friends"  // accepted, mutual
)
