// Package userrole defines the roles a member of a shared space can have.
package userrole

// Role is a member's permission level inside a shared space.
type Role string

const (
	Owner  Role = "owner"
	Viewer Role = "viewer"
)

// IsOwner reports whether the role may write.
func (r Role) IsOwner() bool { return r == Owner }
