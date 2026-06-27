package agent

type StreamDone struct {
	JobPublicID     string `json:"job_public_id"`
	MessagePublicID string `json:"message_public_id"`
}

type StreamStatus struct {
	Text string `json:"text"`
}
