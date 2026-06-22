package job

import "time"

const (
	TypeInitialReportGeneration = "initial_report_generation"
	StatusPending               = "pending"
	StatusRunning               = "running"
	StatusSucceeded             = "succeeded"
	StatusFailed                = "failed"
	QueueInline                 = "inline"
)

type Job struct {
	ID            int64
	PublicID      string
	JobType       string
	Status        string
	QueueName     string
	RelatedType   string
	RelatedID     *int64
	UserID        int64
	InputSummary  map[string]any
	OutputSummary map[string]any
	ErrorMessage  string
	StartedAt     *time.Time
	FinishedAt    *time.Time
}

type ListJobsResult struct {
	Items []JobSummary `json:"items"`
}

type JobSummary struct {
	PublicID      string         `json:"public_id"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	OutputSummary map[string]any `json:"output_summary,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
}
