package response

import "github.com/BloomHealthApp/bloom-health-service/pkg/utils/scoring"

// SummaryResponse is the body of GET /stats/summary.
type SummaryResponse struct {
	CurrentStreak int           `json:"currentStreak"`
	LongestStreak int           `json:"longestStreak"`
	Days          []scoring.Day `json:"days"`
	WorkoutCount  int           `json:"workoutCount"`
	WorkoutSecs   int           `json:"workoutSeconds"`
	Milestones    []int         `json:"milestones"`
}
