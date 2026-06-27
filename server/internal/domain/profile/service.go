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

type summaryRepository interface {
	Summary(ctx context.Context, userID int64) (Summary, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	repo, ok := s.repo.(summaryRepository)
	if !ok {
		return Summary{}, errors.New("profile repository summary dependency is nil")
	}
	summary, err := repo.Summary(ctx, userID)
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

func buildQuickEntries(summary Summary) []QuickEntry {
	profileSummary := "还没有完成形象档案"
	if summary.Profile != nil {
		profileSummary = firstNonEmpty(summary.Profile.StyleGoalSummary, "已完成基础形象档案")
	}

	reportSummary := "暂无初版形象报告"
	if summary.LatestReport != nil {
		reportSummary = firstNonEmpty(summary.LatestReport.Title, "已生成初版形象报告")
	}

	return []QuickEntry{
		{
			Key:     "profile",
			Title:   "形象档案",
			Summary: profileSummary,
		},
		{
			Key:     "memory",
			Title:   "长期记忆",
			Summary: fmt.Sprintf("已沉淀 %d 条事实、%d 条偏好", summary.MemorySummary.FactCount, summary.MemorySummary.PreferenceCount),
		},
		{
			Key:     "report",
			Title:   "形象报告",
			Summary: reportSummary,
		},
		{
			Key:     "feedback",
			Title:   "反馈校准",
			Summary: fmt.Sprintf("%d 条推断待确认", summary.MemorySummary.PendingConfirmationCount),
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
