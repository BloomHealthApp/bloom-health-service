package response

// ChallengeResponse is {"status":"sent","challengeId":"..."} returned by the sign-up and sign-in requests.
type ChallengeResponse struct {
	Status      string `json:"status"`
	ChallengeID string `json:"challengeId"`
}
