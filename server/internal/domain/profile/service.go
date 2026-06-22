package profile

import (
	"context"
	"strings"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/generator"
)

const (
	StatusActive      = "active"
	SourceOnboarding  = "onboarding"
	PolarityPositive  = "positive"
	PolarityNegative  = "negative"
	PrefTypeStyleGoal = "style_goal"
	PrefTypeAvoidance = "avoidance"
)

type Repository interface {
	Upsert(ctx context.Context, item Profile) (Profile, error)
	ReplaceFacts(ctx context.Context, userID int64, profileID int64, facts []Fact) error
	ReplacePrefs(ctx context.Context, userID int64, profileID int64, prefs []Pref) error
	CreateInferences(ctx context.Context, inferences []Inference) error
	MarkUserOnboardingCompleted(ctx context.Context, userID int64) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
