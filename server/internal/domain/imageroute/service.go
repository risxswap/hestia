package imageroute

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/generator"
)

const (
	StatusCandidate  = "candidate"
	StatusActive     = "active"
	StatusArchived   = "archived"
	StatusRefinement = "refinement"

	SourceRule = "rule_generator"

	FeedbackActionLike    = "like"
	FeedbackActionDislike = "dislike"
	FeedbackActionAdjust  = "adjust"

	EventTypeRouteFeedback      = "route_feedback"
	EventSourceOnboardingReport = "onboarding_report"
	EventCreatedByUser          = "user"

	MaxFeedbackReasonLength = 500
)

var (
	ErrRouteNotFound         = errors.New("image route not found")
	ErrInvalidFeedbackAction = errors.New("invalid image route feedback action")
	ErrInvalidFeedbackReason = errors.New("invalid image route feedback reason")
)

type CreateRepository interface {
	CreateMany(ctx context.Context, items []Route) ([]Route, error)
}

type Repository interface {
	CreateRepository
	FeedbackRepository
}

type FeedbackRepository interface {
	FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Route, error)
	UpdateFeedback(ctx context.Context, route Route, event Event) (Route, error)
}

type Service struct {
	repo CreateRepository
}

func NewService(repo CreateRepository) *Service {
	return &Service{repo: repo}
}

type FeedbackService struct {
	repo FeedbackRepository
}

func NewFeedbackService(repo FeedbackRepository) *FeedbackService {
	return &FeedbackService{repo: repo}
}

func (s *Service) CreateFromGenerator(ctx context.Context, userID int64, profileID int64, jobID int64, routes []generator.RouteResult) ([]Route, error) {
	if len(routes) == 0 {
		return []Route{}, nil
	}
	items := make([]Route, 0, len(routes))
	for _, route := range routes {
		items = append(items, Route{
			PublicID:         id.NewPublicID("irt"),
			UserID:           userID,
			ProfileID:        profileID,
			Name:             route.Name,
			RouteRole:        route.Role,
			Status:           StatusCandidate,
			Weight:           route.Weight,
			TargetImpression: route.TargetImpression,
			SuitableScenes:   route.SuitableScenes,
			HairStrategy:     strategyMap(route.HairStrategy),
			MakeupStrategy:   strategyMap(route.MakeupStrategy),
			OutfitStrategy:   strategyMap(route.OutfitStrategy),
			AvoidPoints:      route.AvoidPoints,
			Reason:           route.Reason,
			Source:           SourceRule,
			CreatedFromJobID: jobID,
		})
	}
	return s.repo.CreateMany(ctx, items)
}

func (s *FeedbackService) ApplyFeedback(ctx context.Context, userID int64, routePublicID string, input FeedbackInput) (FeedbackResponse, error) {
	input = normalizeFeedbackInput(input)
	if utf8.RuneCountInString(input.Reason) > MaxFeedbackReasonLength {
		return FeedbackResponse{}, ErrInvalidFeedbackReason
	}
	status, err := statusForFeedbackAction(input.Action)
	if err != nil {
		return FeedbackResponse{}, err
	}
	route, err := s.repo.FindByPublicIDForUser(ctx, userID, routePublicID)
	if err != nil {
		return FeedbackResponse{}, err
	}
	route.Status = status
	if input.Action == FeedbackActionLike && route.ActivatedAt == nil {
		now := time.Now().UTC()
		route.ActivatedAt = &now
	}
	event := Event{
		ImageRouteID: route.ID,
		UserID:       userID,
		EventType:    EventTypeRouteFeedback,
		EventValue: map[string]any{
			"action": input.Action,
			"reason": input.Reason,
			"status": status,
		},
		Source:    EventSourceOnboardingReport,
		CreatedBy: EventCreatedByUser,
	}
	updated, err := s.repo.UpdateFeedback(ctx, route, event)
	if err != nil {
		return FeedbackResponse{}, err
	}
	return FeedbackResponse{
		PublicID: updated.PublicID,
		Status:   updated.Status,
	}, nil
}

func normalizeFeedbackInput(input FeedbackInput) FeedbackInput {
	input.Action = strings.TrimSpace(input.Action)
	input.Reason = strings.TrimSpace(input.Reason)
	return input
}

func statusForFeedbackAction(action string) (string, error) {
	switch action {
	case FeedbackActionLike:
		return StatusActive, nil
	case FeedbackActionDislike:
		return StatusArchived, nil
	case FeedbackActionAdjust:
		return StatusRefinement, nil
	default:
		return "", ErrInvalidFeedbackAction
	}
}

func strategyMap(strategy generator.StrategyBlock) map[string]any {
	return map[string]any{
		"direction":    strategy.Direction,
		"steps":        strategy.Steps,
		"alternatives": strategy.Alternatives,
	}
}
