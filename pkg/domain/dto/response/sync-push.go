package response

// SyncResult is one entry of a push response.
type SyncResult struct {
	OperationID string         `json:"operationId"`
	Status      string         `json:"status"`
	Entity      map[string]any `json:"entity,omitempty"`
	EntityType  string         `json:"entityType,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Resolved    string         `json:"resolved,omitempty"`
}

// SyncPushResponse is the body of POST /sync/push.
type SyncPushResponse struct {
	Results []SyncResult `json:"results"`
}
