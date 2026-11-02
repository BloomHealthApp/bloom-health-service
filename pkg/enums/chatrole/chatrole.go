// Package chatrole enumerates the allowed values of a chat message's role.
package chatrole

const (
	User      = "user"
	Assistant = "assistant"
)

func Values() []string { return []string{User, Assistant} }

func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
