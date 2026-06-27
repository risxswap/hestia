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
	Summary(ctx context.Context, userID int64) (Summary, error)
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
