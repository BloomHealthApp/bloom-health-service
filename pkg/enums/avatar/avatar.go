// Package avatar enumerates the allowed profile avatar ids (bubu-01 ... bubu-09).
package avatar

import "fmt"

// Count is the number of available avatars.
const Count = 9

// All returns every allowed avatar id in order.
func All() []string {
	ids := make([]string, 0, Count)
	for i := 1; i <= Count; i++ {
		ids = append(ids, fmt.Sprintf("bubu-%02d", i))
	}
	return ids
}

// IsValid reports whether s is an allowed avatar id (exact match, case-sensitive).
func IsValid(s string) bool {
	for _, v := range All() {
		if v == s {
			return true
		}
	}
	return false
}
