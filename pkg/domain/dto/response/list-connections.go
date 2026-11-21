package response

// ConnectionResponse is one friend or friend request. Profiles in lists carry no stats.
type ConnectionResponse struct {
	ID      string                `json:"id"`
	Profile PublicProfileResponse `json:"profile"`
	// Since is when the request was sent, or for friends when it was accepted (RFC3339).
	Since string `json:"since"`
}

// ConnectionsResponse is GET /connections, each list newest first.
type ConnectionsResponse struct {
	Friends  []ConnectionResponse `json:"friends"`
	Incoming []ConnectionResponse `json:"incoming"`
	Outgoing []ConnectionResponse `json:"outgoing"`
}
