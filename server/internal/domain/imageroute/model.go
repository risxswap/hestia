package imageroute

import "time"

type Route struct {
	ID                  int64
	PublicID            string
	UserID              int64
	ProfileID           int64
	Name                string
	RouteRole           string
	Status              string
	Weight              float64
	TargetImpression    []string
	SuitableScenes      []string
	HairStrategy        map[string]any
	MakeupStrategy      map[string]any
	OutfitStrategy      map[string]any
	AvoidPoints         []string
	Reason              []string
	Source              string
	CreatedFromReportID *int64
	CreatedFromJobID    int64
	ActivatedAt         *time.Time
}

type Event struct {
	ID           int64
	ImageRouteID int64
	UserID       int64
	EventType    string
	EventValue   map[string]any
	Source       string
	CreatedBy    string
}

type FeedbackInput struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type FeedbackResponse struct {
	PublicID string `json:"public_id"`
	Status   string `json:"status"`
}
