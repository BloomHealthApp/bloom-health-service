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
