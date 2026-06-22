package job

type ListJobsResult struct {
	Items []JobSummary `json:"items"`
}

type JobSummary struct {
	PublicID string `json:"public_id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}
