package profile

type Profile struct {
	ID                 int64
	PublicID           string
	UserID             int64
	Status             string
	Gender             string
	HeightCM           *int
	BodyNotes          string
	SkinNotes          string
	HairNotes          string
	LifestyleScenarios []string
	StyleGoalSummary   string
}

type Fact struct {
	Key    string
	Value  any
	Source string
}

type Pref struct {
	Type       string
	Key        string
	Value      any
	Polarity   string
	Source     string
	Confidence *float64
}

type Inference struct {
	UserID      int64
	ProfileID   int64
	Key         string
	Value       any
	Confidence  *float64
	SourceJobID int64
}

type OnboardingInput struct {
	Gender             string
	HeightCM           *int
	BodyNotes          string
	SkinNotes          string
	HairNotes          string
	LifestyleScenarios []string
	StyleGoals         []string
	Avoidances         []string
}
