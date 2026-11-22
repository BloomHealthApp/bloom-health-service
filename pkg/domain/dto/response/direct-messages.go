package response

import "time"

// DMThreadsResponse is GET /dm/threads: your chats with friends, newest message first.
type DMThreadsResponse struct {
	Items       []DMThreadResponse `json:"items"`
	UnreadCount int                `json:"unreadCount"`
}

// DMThreadResponse is one chat in the inbox. Friend is only ever a public profile.
type DMThreadResponse struct {
	Friend PublicProfileResponse `json:"friend"`
	Last   *DMLastResponse       `json:"last"`
	Unread bool                  `json:"unread"`
}

type DMLastResponse struct {
	Text      string    `json:"text"`
	Mine      bool      `json:"mine"`
	CreatedAt time.Time `json:"createdAt"`
}

// DMResponse is one message.
type DMResponse struct {
	ID        string    `json:"id"`
	Seq       int64     `json:"seq"`
	SenderID  string    `json:"senderId"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// DMPageResponse is GET /dm/:userId/messages, oldest first. SeenAt is when the friend last read the chat.
type DMPageResponse struct {
	Items      []DMResponse `json:"items"`
	HasMore    bool         `json:"hasMore"`
	NextBefore *int64       `json:"nextBefore"`
	SeenAt     *time.Time   `json:"seenAt"`
}
