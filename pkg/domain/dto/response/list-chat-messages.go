package response

// ChatMessagesResponse is a page of GET /chat/sessions/:sid/messages, oldest first.
type ChatMessagesResponse struct {
	Items      []ChatMessageResponse `json:"items"`
	HasMore    bool                  `json:"hasMore"`
	NextBefore *int                  `json:"nextBefore"`
}
