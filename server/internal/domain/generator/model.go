package generator

import "context"

type ReportGenerator interface {
	GenerateInitialReport(ctx context.Context, input InitialReportInput) (*InitialReportResult, error)
}

type InitialReportInput struct {
	UserID          int64
	ProfileID       int64
	StyleGoals      []string
	Avoidances      []string
	Scenarios       []string
	Basic           map[string]any
	BodyNotes       string
	SkinNotes       string
	HairNotes       string
	WardrobeItems   []WardrobeItemInput
	ReferenceStyles []string
}

type WardrobeItemInput struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Color      string `json:"color,omitempty"`
	Silhouette string `json:"silhouette,omitempty"`
	Material   string `json:"material,omitempty"`
	Season     string `json:"season,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type InitialReportResult struct {
	Summary              string                  `json:"summary"`
	Routes               []RouteResult           `json:"routes"`
	HairStrategy         StrategyBlock           `json:"hair_strategy"`
	MakeupStrategy       StrategyBlock           `json:"makeup_strategy"`
	OutfitStrategy       StrategyBlock           `json:"outfit_strategy"`
	Avoidances           []string                `json:"avoidances"`
	WardrobeCombinations []WardrobeCombination   `json:"wardrobe_combinations"`
	WardrobeGaps         []WardrobeGap           `json:"wardrobe_gaps"`
	ActionItems          []string                `json:"action_items"`
	ReferenceStyleLogic  string                  `json:"reference_style_logic"`
	PrivacyNote          string                  `json:"privacy_note"`
	Inferences           []ProfileInferenceInput `json:"inferences,omitempty"`
}

type RouteResult struct {
	Name             string        `json:"name"`
	Role             string        `json:"role"`
	Weight           float64       `json:"weight"`
	TargetImpression []string      `json:"target_impression"`
	SuitableScenes   []string      `json:"suitable_scenes"`
	HairStrategy     StrategyBlock `json:"hair_strategy"`
	MakeupStrategy   StrategyBlock `json:"makeup_strategy"`
	OutfitStrategy   StrategyBlock `json:"outfit_strategy"`
	AvoidPoints      []string      `json:"avoid_points"`
	Reason           []string      `json:"reason"`
}

type StrategyBlock struct {
	Direction    string   `json:"direction"`
	Steps        []string `json:"steps"`
	Alternatives []string `json:"alternatives,omitempty"`
}

type WardrobeCombination struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
	Scene string   `json:"scene"`
	Why   string   `json:"why"`
}

type WardrobeGap struct {
	GapType     string `json:"gap_type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Priority    int    `json:"priority"`
}

type ProfileInferenceInput struct {
	Key        string  `json:"key"`
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}
