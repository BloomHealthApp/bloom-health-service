package bo

// ProfileStats are the only activity numbers a friend may see: no days, rest days or anything period-related.
type ProfileStats struct {
	CurrentStreak int
	LongestStreak int
	Habits        int
}

// PublicCard is what the unauthenticated profile page may show for a link: a first name and an initial, nothing else.
type PublicCard struct {
	FirstName string
	Initial   string
}
