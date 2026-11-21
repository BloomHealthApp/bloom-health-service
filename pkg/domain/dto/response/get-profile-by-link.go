package response

// ProfileByLinkResponse is GET /profiles/by-link/{code} and POST /connections. Stats are included only for friends
// and for the caller's own link.
type ProfileByLinkResponse struct {
	Profile      PublicProfileResponse `json:"profile"`
	Relation     string                `json:"relation"`
	ConnectionID *string               `json:"connectionId,omitempty"`
}
