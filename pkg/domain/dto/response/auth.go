package response

// AuthResponse is returned by OTP verify and refresh.
type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
}
