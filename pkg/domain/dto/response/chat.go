package response

// ChatModelResponse is one entry of GET /chat/models.
type ChatModelResponse struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Vendor  string `json:"vendor"`
	Default bool   `json:"default"`
}

// ChatModelsResponse is the body of GET /chat/models.
type ChatModelsResponse struct {
	Items []ChatModelResponse `json:"items"`
}

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

// ChatSessionsResponse is the body of GET /chat/sessions.
type ChatSessionsResponse struct {
	Items []ChatSessionResponse `json:"items"`
}

// ChatMessageResponse is one chat message. Status is pending|succeeded|failed.
type ChatMessageResponse struct {
	ID        string  `json:"id"`
	SessionID string  `json:"sessionId"`
	Idx       int     `json:"idx"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	Status    string  `json:"status"`
	ModelID   string  `json:"modelId"`
	ErrorCode *string `json:"errorCode"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

// ChatMessagesResponse is a page of GET /chat/sessions/:sid/messages, oldest first.
type ChatMessagesResponse struct {
	Items      []ChatMessageResponse `json:"items"`
	HasMore    bool                  `json:"hasMore"`
	NextBefore *int                  `json:"nextBefore"`
}

// SendChatMessageResponse is the 202 body of POST /chat/sessions/:sid/messages.
type SendChatMessageResponse struct {
	UserMessage      ChatMessageResponse `json:"userMessage"`
	AssistantMessage ChatMessageResponse `json:"assistantMessage"`
}
