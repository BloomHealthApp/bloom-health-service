package dao

import (
	"time"
)

// Entity is a versioned, soft-deletable record scoped to a shared space. Type-specific fields live in
// Data and are validated by infrastructure/validation.
type Entity struct {
	ID        string
	Type      string
	SpaceID   string
	Version   int
	Seq       int64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	Data      map[string]any
}

// Public renders the entity as the API representation.
func (e *Entity) Public() map[string]any {
	out := make(map[string]any, len(e.Data)+6)
	for k, v := range e.Data {
		out[k] = v
	}
	out["id"] = e.ID
	out["version"] = e.Version
	out["createdAt"] = e.CreatedAt.UTC().Format(time.RFC3339)
	out["updatedAt"] = e.UpdatedAt.UTC().Format(time.RFC3339)
	if e.DeletedAt != nil {
		out["deletedAt"] = e.DeletedAt.UTC().Format(time.RFC3339)
	} else {
		out["deletedAt"] = nil
	}
	return out
}

// Clone deep-copies the top level of the entity.
func (e *Entity) Clone() *Entity {
	c := *e
	c.Data = make(map[string]any, len(e.Data))
	for k, v := range e.Data {
		c.Data[k] = v
	}
	if e.DeletedAt != nil {
		t := *e.DeletedAt
		c.DeletedAt = &t
	}
	return &c
}
