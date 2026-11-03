package response

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
