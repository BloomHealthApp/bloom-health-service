package response

// ChatSessionResponse is the contract's chat session object, shared by the session endpoints.
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
