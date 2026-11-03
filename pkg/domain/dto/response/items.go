package response

// ItemsResponse wraps list endpoints.
type ItemsResponse struct {
	Items []map[string]any `json:"items"`
}
