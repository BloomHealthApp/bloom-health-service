// Package periodsymptom enumerates the allowed symptoms of a period entry.
package periodsymptom

const (
	Cramps        = "cramps"
	Headache      = "headache"
	Bloating      = "bloating"
	Fatigue       = "fatigue"
	MoodSwings    = "mood_swings"
	Backache      = "backache"
	Acne          = "acne"
	Nausea        = "nausea"
	Cravings      = "cravings"
	TenderBreasts = "tender_breasts"
)

// Values lists every allowed value.
func Values() []string {
	return []string{Cramps, Headache, Bloating, Fatigue, MoodSwings, Backache, Acne, Nausea, Cravings, TenderBreasts}
}

// Valid reports whether s is an allowed value (exact, case-sensitive).
func Valid(s string) bool {
	for _, v := range Values() {
		if v == s {
			return true
		}
	}
	return false
}
