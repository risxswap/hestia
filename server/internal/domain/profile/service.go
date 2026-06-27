package profile

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/generator"
)

const (
	StatusActive               = "active"
	SourceOnboarding           = "onboarding"
	SourceUser                 = "user"
	PolarityPositive           = "positive"
	PolarityNegative           = "negative"
	PrefTypeStyleGoal          = "style_goal"
	PrefTypeAvoidance          = "avoidance"
	PrefTypeScenarioPreference = "scenario_preference"
)

const (
	maxProfileTextLength = 220
	maxScenarioCount     = 8
	maxScenarioLength    = 40
	maxPreferenceCount   = 12
	maxPreferenceLength  = 60
	minHeightCM          = 80
	maxHeightCM          = 250
)

type Repository interface {
	Summary(ctx context.Context, userID int64) (Summary, error)
	UpdateExplicitProfile(ctx context.Context, userID int64, input UpdateProfileInput) (Summary, error)
	UpdateExplicitPreferences(ctx context.Context, userID int64, input UpdatePreferencesInput) (Summary, error)
	Upsert(ctx context.Context, item Profile) (Profile, error)
	ReplaceFacts(ctx context.Context, userID int64, profileID int64, facts []Fact) error
	ReplacePrefs(ctx context.Context, userID int64, profileID int64, prefs []Pref) error
	CreateInferences(ctx context.Context, inferences []Inference) error
	MarkUserOnboardingCompleted(ctx context.Context, userID int64) error
}

type Service struct {
	repo Repository
}

var ErrValidation = errors.New("profile validation failed")
var ErrUserNotFound = errors.New("profile user not found")

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return ErrValidation.Error()
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e ValidationError) Is(target error) bool {
	return target == ErrValidation
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	summary, err := s.repo.Summary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, request UpdateProfileRequest) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateUpdateProfile(request)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.repo.UpdateExplicitProfile(ctx, userID, input)
	if err != nil {
		return Summary{}, err
	}
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, userID int64, request UpdatePreferencesRequest) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	input, err := validateUpdatePreferences(request)
	if err != nil {
		return Summary{}, err
	}
	summary, err := s.repo.UpdateExplicitPreferences(ctx, userID, input)
	if err != nil {
		return Summary{}, err
	}
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}

func (s *Service) UpsertFromOnboarding(ctx context.Context, userID int64, input OnboardingInput) (Profile, error) {
	profile := Profile{
		PublicID:           id.NewPublicID("prf"),
		UserID:             userID,
		Status:             StatusActive,
		Gender:             input.Gender,
		HeightCM:           input.HeightCM,
		BodyNotes:          input.BodyNotes,
		SkinNotes:          input.SkinNotes,
		HairNotes:          input.HairNotes,
		LifestyleScenarios: input.LifestyleScenarios,
		StyleGoalSummary:   strings.Join(input.StyleGoals, "、"),
	}
	created, err := s.repo.Upsert(ctx, profile)
	if err != nil {
		return Profile{}, err
	}

	facts := []Fact{
		{Key: "gender", Value: input.Gender, Source: SourceOnboarding},
		{Key: "height_cm", Value: input.HeightCM, Source: SourceOnboarding},
		{Key: "lifestyle_scenarios", Value: input.LifestyleScenarios, Source: SourceOnboarding},
	}
	if err := s.repo.ReplaceFacts(ctx, userID, created.ID, facts); err != nil {
		return Profile{}, err
	}

	prefs := make([]Pref, 0, len(input.StyleGoals)+len(input.Avoidances))
	for _, goal := range input.StyleGoals {
		if strings.TrimSpace(goal) == "" {
			continue
		}
		prefs = append(prefs, Pref{Type: PrefTypeStyleGoal, Key: goal, Value: goal, Polarity: PolarityPositive, Source: SourceOnboarding})
	}
	for _, avoidance := range input.Avoidances {
		if strings.TrimSpace(avoidance) == "" {
			continue
		}
		prefs = append(prefs, Pref{Type: PrefTypeAvoidance, Key: avoidance, Value: avoidance, Polarity: PolarityNegative, Source: SourceOnboarding})
	}
	if err := s.repo.ReplacePrefs(ctx, userID, created.ID, prefs); err != nil {
		return Profile{}, err
	}
	return created, nil
}

func (s *Service) SaveGeneratorInferences(ctx context.Context, userID int64, profileID int64, sourceJobID int64, inputs []generator.ProfileInferenceInput) error {
	if len(inputs) == 0 {
		return nil
	}
	inferences := make([]Inference, 0, len(inputs))
	for _, input := range inputs {
		confidence := input.Confidence
		inferences = append(inferences, Inference{
			UserID:      userID,
			ProfileID:   profileID,
			Key:         input.Key,
			Value:       input.Value,
			Confidence:  &confidence,
			SourceJobID: sourceJobID,
		})
	}
	return s.repo.CreateInferences(ctx, inferences)
}

func (s *Service) CompleteOnboarding(ctx context.Context, userID int64) error {
	return s.repo.MarkUserOnboardingCompleted(ctx, userID)
}

func validateUpdateProfile(request UpdateProfileRequest) (UpdateProfileInput, error) {
	input := UpdateProfileInput{
		Nickname:  sanitizePatchString(request.Nickname),
		Gender:    sanitizePatchString(request.Gender),
		HeightCM:  request.HeightCM,
		BodyNotes: sanitizePatchString(request.BodyNotes),
		SkinNotes: sanitizePatchString(request.SkinNotes),
		HairNotes: sanitizePatchString(request.HairNotes),
	}
	if err := validatePatchString(input.Nickname, "nickname", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.Gender, "gender", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.BodyNotes, "body_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.SkinNotes, "skin_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if err := validatePatchString(input.HairNotes, "hair_notes", maxProfileTextLength); err != nil {
		return UpdateProfileInput{}, err
	}
	if input.HeightCM.Present && input.HeightCM.Value != nil && (*input.HeightCM.Value < minHeightCM || *input.HeightCM.Value > maxHeightCM) {
		return UpdateProfileInput{}, ValidationError{Field: "height_cm", Message: "out of range"}
	}
	if request.LifestyleScenarios.Present {
		scenarios, err := sanitizeStringList(request.LifestyleScenarios.Value, maxScenarioCount, maxScenarioLength, "lifestyle_scenarios")
		if err != nil {
			return UpdateProfileInput{}, err
		}
		input.LifestyleScenarios = PatchStringSlice{Present: true, Value: scenarios}
	}
	return input, nil
}

func sanitizePatchString(value PatchString) PatchString {
	if !value.Present {
		return value
	}
	value.Value = strings.TrimSpace(value.Value)
	return value
}

func validatePatchString(value PatchString, field string, maxLength int) error {
	if !value.Present {
		return nil
	}
	if len([]rune(value.Value)) > maxLength {
		return ValidationError{Field: field, Message: "too long"}
	}
	return nil
}

func validateUpdatePreferences(request UpdatePreferencesRequest) (UpdatePreferencesInput, error) {
	styleGoals, err := sanitizeStringList(request.StyleGoals, maxPreferenceCount, maxPreferenceLength, "style_goals")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	avoidances, err := sanitizeStringList(request.Avoidances, maxPreferenceCount, maxPreferenceLength, "avoidances")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	scenarios, err := sanitizeStringList(request.ScenarioPreferences, maxPreferenceCount, maxPreferenceLength, "scenario_preferences")
	if err != nil {
		return UpdatePreferencesInput{}, err
	}
	return UpdatePreferencesInput{
		StyleGoals:          styleGoals,
		Avoidances:          avoidances,
		ScenarioPreferences: scenarios,
	}, nil
}

func sanitizeStringList(values []string, maxCount int, maxLength int, field string) ([]string, error) {
	if len(values) > maxCount {
		return nil, ValidationError{Field: field, Message: "too many"}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if len([]rune(trimmed)) > maxLength {
			return nil, ValidationError{Field: field, Message: "too long"}
		}
		result = append(result, trimmed)
	}
	return result, nil
}

func buildQuickEntries(summary Summary) []QuickEntry {
	scenarioCount := 0
	if summary.Profile != nil {
		scenarioCount = len(summary.Profile.LifestyleScenarios)
	}
	profileSummary := "还没有记录常见场景"
	if scenarioCount > 0 {
		profileSummary = fmt.Sprintf("已记录 %d 个常见场景", scenarioCount)
	}

	reportSummary := "暂无初版报告"
	if summary.LatestReport != nil {
		reportSummary = "初版报告已生成"
	}

	return []QuickEntry{
		{
			Key:     "profile",
			Title:   "我的档案",
			Summary: profileSummary,
		},
		{
			Key:     "preferences",
			Title:   "偏好与禁忌",
			Summary: fmt.Sprintf("%d 个风格目标、%d 个禁忌", summary.MemorySummary.PreferenceCount, summary.MemorySummary.AvoidanceCount),
		},
		{
			Key:     "report",
			Title:   "报告与路线",
			Summary: reportSummary,
		},
		{
			Key:     "privacy",
			Title:   "隐私与数据",
			Summary: "照片、档案、反馈可管理",
		},
	}
}
