package response

// HealthResponse is the body of GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Time    string `json:"time"`
	Version string `json:"version"`
}
