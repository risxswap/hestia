package imageroute

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
}
