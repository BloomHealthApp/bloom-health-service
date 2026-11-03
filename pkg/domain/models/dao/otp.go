package dao

import (
	"time"
)

// OTP challenge purposes.
const (
	OTPPurposeSignUp = "signup"
	OTPPurposeSignIn = "signin"
	// OTPPurposePhoneChange proves a signed-in user controls the account email before a phone is saved.
	OTPPurposePhoneChange = "phone_change"
)

// OTPChallenge is a pending one-time code (stored hashed), addressed by its opaque ChallengeID.
// Sign-up challenges carry the pending name and phone; the account exists only after verification.
type OTPChallenge struct {
	ChallengeID   string    `bson:"_id"`
	Purpose       string    `bson:"purpose"`
	Email         string    `bson:"email"` // lower-cased address the code was mailed to
	PendingName   string    `bson:"pendingName,omitempty"`
	PendingPhone  string    `bson:"pendingPhone,omitempty"`
	PendingAvatar string    `bson:"pendingAvatar,omitempty"` // optional avatar id chosen at sign-up
	PendingGender string    `bson:"pendingGender,omitempty"` // optional gender chosen at sign-up
	CodeHash      string    `bson:"codeHash"`
	CreatedAt     time.Time `bson:"createdAt"`
	ExpiresAt     time.Time `bson:"expiresAt"`
	Attempts      int       `bson:"attempts"`
}
