package response

// SyncPullResponse is the body of GET /sync/pull.
type SyncPullResponse struct {
	Cursor  string                      `json:"cursor"`
	HasMore bool                        `json:"hasMore"`
	Changes map[string][]map[string]any `json:"changes"`
}
