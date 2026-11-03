package response

// ChatSessionResponse is a chat session.
type ChatSessionResponse struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	ModelID       string   `json:"modelId"`
	ContextScopes []string `json:"contextScopes"`
	// PendingMessageID is the assistant message being answered, or null.
	PendingMessageID *string `json:"pendingMessageId"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}
