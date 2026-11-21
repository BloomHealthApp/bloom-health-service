package response

// ProfileLinkResponse is GET /me/profile-link and POST /me/profile-link/reset.
type ProfileLinkResponse struct {
	Code string `json:"code"`
	URL  string `json:"url"`
}
