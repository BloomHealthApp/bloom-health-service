// Package response holds the response DTOs. JSON field names follow docs/api-contract.md.
package response

import "github.com/BloomHealthApp/bloom-health-service/pkg/utils/scoring"

// UserResponse is the `user` object of the contract.
type UserResponse struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Phone            *string `json:"phone"` // E.164; null for legacy accounts
	DisplayName      string  `json:"displayName"`
	Avatar           *string `json:"avatar"`
	Timezone         string  `json:"timezone"`
	Theme            string  `json:"theme"`
	Role             string  `json:"role"`
	SharedSpaceID    string  `json:"sharedSpaceId"`
	OwnerDisplayName string  `json:"ownerDisplayName"`
	OwnerAvatar      *string `json:"ownerAvatar"` // the space owner's avatar (own avatar for an owner)
	Gender           *string `json:"gender"`      // female|male|other|prefer_not_to_say|null
	// PeriodTrackerEnabled is the effective setting: explicit user choice, else gender == female.
	PeriodTrackerEnabled bool `json:"periodTrackerEnabled"`
}

// AuthResponse is returned by OTP verify and refresh.
type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
}

// StatusResponse is {"status": "..."}.
type StatusResponse struct {
	Status string `json:"status"`
}

// ChallengeResponse is {"status":"sent","challengeId":"..."} returned by the sign-up and sign-in requests.
type ChallengeResponse struct {
	Status      string `json:"status"`
	ChallengeID string `json:"challengeId"`
}

// ItemsResponse wraps list endpoints.
type ItemsResponse struct {
	Items []map[string]any `json:"items"`
}

// SyncResult is one entry of a push response.
type SyncResult struct {
	OperationID string         `json:"operationId"`
	Status      string         `json:"status"`
	Entity      map[string]any `json:"entity,omitempty"`
	EntityType  string         `json:"entityType,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Resolved    string         `json:"resolved,omitempty"`
}

// SyncPushResponse is the body of POST /sync/push.
type SyncPushResponse struct {
	Results []SyncResult `json:"results"`
}

// SyncPullResponse is the body of GET /sync/pull.
type SyncPullResponse struct {
	Cursor  string                      `json:"cursor"`
	HasMore bool                        `json:"hasMore"`
	Changes map[string][]map[string]any `json:"changes"`
}

// SummaryResponse is the body of GET /stats/summary.
type SummaryResponse struct {
	CurrentStreak int           `json:"currentStreak"`
	LongestStreak int           `json:"longestStreak"`
	Days          []scoring.Day `json:"days"`
	WorkoutCount  int           `json:"workoutCount"`
	WorkoutSecs   int           `json:"workoutSeconds"`
	Milestones    []int         `json:"milestones"`
}

// HealthResponse is the body of GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Time    string `json:"time"`
	Version string `json:"version"`
}
