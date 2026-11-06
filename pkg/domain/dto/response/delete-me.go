package response

// DeletionScheduledResponse is DELETE /v1/me: the account is locked and will be erased at DeleteAfter (RFC3339).
type DeletionScheduledResponse struct {
	DeleteAfter string `json:"deleteAfter"`
}
