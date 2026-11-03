package response

// ChatSessionsResponse is the body of GET /chat/sessions.
type ChatSessionsResponse struct {
	Items []ChatSessionResponse `json:"items"`
}
