package request

import (
	"bytes"
	"encoding/json"
)

// OptString distinguishes an absent field from an explicit JSON null.
type OptString struct {
	Set   bool
	Null  bool
	Value string
}

// UnmarshalJSON records presence; non-string, non-null values are decode errors (400).
func (o *OptString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}

// OptBool distinguishes an absent field from an explicit JSON null.
type OptBool struct {
	Set   bool
	Null  bool
	Value bool
}

// UnmarshalJSON records presence; non-bool, non-null values are decode errors (400).
func (o *OptBool) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}
