// Package dao holds the persisted documents. Simple aggregates map directly to BSON; entities and
// operation records are converted by the Mongo repositories because they embed free-form data.
package dao

import (
	"time"
)

// User is an account, created on first verified login.
type User struct {
	ID         string `bson:"_id"`
	Email      string `bson:"email"`
	EmailLower string `bson:"emailLower"`
	// Phone is E.164 and unique; nil (field absent) for legacy accounts created before phone sign-up.
	Phone       *string `bson:"phone,omitempty"`
	DisplayName string  `bson:"displayName"`
	Avatar      *string `bson:"avatar"`
	// Gender is one of pkg/enums/gender; nil (null) for legacy accounts or when not provided.
	Gender *string `bson:"gender,omitempty"`
	// PeriodTracker is the explicit period-tracker setting; nil means "default for the gender"
	// (on for female). Use IsPeriodTrackerEnabled for the effective value.
	PeriodTracker *bool     `bson:"periodTracker,omitempty"`
	Timezone      string    `bson:"timezone"`
	Theme         string    `bson:"theme"`
	CreatedAt     time.Time `bson:"createdAt"`
	UpdatedAt     time.Time `bson:"updatedAt"`
	// DeletionRequestedAt and DeleteAfter are set while the account is scheduled for deletion (DELETE /v1/me).
	// The account is locked until it is restored, and erased once DeleteAfter has passed.
	DeletionRequestedAt *time.Time `bson:"deletionRequestedAt,omitempty"`
	DeleteAfter         *time.Time `bson:"deleteAfter,omitempty"`
}

// PendingDeletion reports whether the account is scheduled for deletion.
func (u *User) PendingDeletion() bool { return u != nil && u.DeleteAfter != nil }

// IsPeriodTrackerEnabled is the effective tracker setting: the explicit choice if set, else gender == female.
func (u *User) IsPeriodTrackerEnabled() bool {
	if u == nil {
		return false
	}
	if u.PeriodTracker != nil {
		return *u.PeriodTracker
	}
	return u.Gender != nil && *u.Gender == "female"
}
