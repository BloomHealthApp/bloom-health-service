package response

// PublicProfileResponse is the only shape another user's profile is ever returned in. Never add email, phone,
// gender, tracker or period fields.
type PublicProfileResponse struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"displayName"`
	Avatar      *string              `json:"avatar"`
	Stats       *PublicStatsResponse `json:"stats,omitempty"`
}

// PublicStatsResponse is shown to friends and to the user themself; absent for accounts without a space of their own.
type PublicStatsResponse struct {
	CurrentStreak int `json:"currentStreak"`
	LongestStreak int `json:"longestStreak"`
	Habits        int `json:"habits"`
}
