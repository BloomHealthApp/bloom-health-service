package response

// SendChatMessageResponse is the 202 body of POST /chat/sessions/:sid/messages.
type SendChatMessageResponse struct {
	UserMessage      ChatMessageResponse `json:"userMessage"`
	AssistantMessage ChatMessageResponse `json:"assistantMessage"`
}
