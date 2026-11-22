package bo

// ProfileStats are the streak numbers on a public profile: never rest days or anything period-related.
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

// ProfileActivity is what a friend sees of someone's progress: how full each day was and today's habits.
// Days carry a heat level only, never rest-day flags, so nothing hints at why a day was quiet.
type ProfileActivity struct {
	Days        []ActivityDay
	TodayHabits []TodayHabit
}

// ActivityDay is a past day with some progress; days with none are left out.
type ActivityDay struct {
	Date  string
	Level int
}

type TodayHabit struct {
	Title string
	Icon  string
	Done  bool
}
