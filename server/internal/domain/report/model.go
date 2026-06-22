package report

import "time"

const (
	TypeInitial      = "initial"
	StatusGenerating = "generating"
	StatusReady      = "ready"
)

type Report struct {
	ID              int64
	PublicID        string
	UserID          int64
	ProfileID       int64
	ReportType      string
	Status          string
	Title           string
	Summary         string
	ContentJSON     map[string]any
	ContextSnapshot map[string]any
	StyleRefsJSON   map[string]any
	JobID           int64
	GeneratedAt     *time.Time
}

type ReportRoute struct {
	ImageRouteID  int64
	RouteRole     string
	SortOrder     int
	RouteSnapshot map[string]any
}

type CreateInitialInput struct {
	UserID          int64
	ProfileID       int64
	JobID           int64
	Title           string
	Summary         string
	ContentJSON     map[string]any
	ContextSnapshot map[string]any
	StyleRefsJSON   map[string]any
}

type Response struct {
	PublicID    string         `json:"public_id"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary"`
	ContentJSON map[string]any `json:"content_json"`
	Routes      []any          `json:"routes"`
}
