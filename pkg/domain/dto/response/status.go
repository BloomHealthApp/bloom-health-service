package response

// StatusResponse is {"status": "..."}.
type StatusResponse struct {
	Status string `json:"status"`
}
