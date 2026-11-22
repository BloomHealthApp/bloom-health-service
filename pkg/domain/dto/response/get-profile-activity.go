package response

// ProfileActivityResponse is GET /profiles/{id}/activity: a friend's (or the caller's own) year of progress and
// today's habits. Never add rest days, period, log values, notes or workouts.
type ProfileActivityResponse struct {
	Days        []ProfileActivityDay `json:"days"`
	TodayHabits []ProfileTodayHabit  `json:"todayHabits"`
}

// ProfileActivityDay is a day with progress; level is 1-4 as in the app's activity graph. Missing days are 0.
type ProfileActivityDay struct {
	Date  string `json:"date"`
	Level int    `json:"level"`
}

type ProfileTodayHabit struct {
	Title string `json:"title"`
	Icon  string `json:"icon"`
	Done  bool   `json:"done"`
}
