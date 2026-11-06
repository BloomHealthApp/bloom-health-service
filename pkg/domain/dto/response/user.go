// Package response holds the response DTOs. JSON field names follow docs/api-contract.md.
package response

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
	// DeletionScheduledAt is when the account will be erased (RFC3339); null unless deletion was requested.
	DeletionScheduledAt *string `json:"deletionScheduledAt"`
}
