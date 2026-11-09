package response

// AuthResponse is returned by OTP verify, Google sign-in and refresh.
type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
	// IsNewUser is set only by Google sign-in, when it just created the account.
	IsNewUser bool `json:"isNewUser,omitempty"`
}
