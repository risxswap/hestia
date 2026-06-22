package imageroute

import (
	"context"
	"errors"
	"time"

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
)

var (
	ErrRouteNotFound         = errors.New("image route not found")
	ErrInvalidFeedbackAction = errors.New("invalid image route feedback action")
)

type Repository interface {
	CreateMany(ctx context.Context, items []Route) ([]Route, error)
}

type FeedbackRepository interface {
	FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Route, error)
	UpdateFeedback(ctx context.Context, route Route, event Event) (Route, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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

func (s *Service) ApplyFeedback(ctx context.Context, userID int64, routePublicID string, input FeedbackInput) (FeedbackResponse, error) {
	status, err := statusForFeedbackAction(input.Action)
	if err != nil {
		return FeedbackResponse{}, err
	}
	repo, ok := s.repo.(FeedbackRepository)
	if !ok {
		return FeedbackResponse{}, errors.New("image route feedback repository is unavailable")
	}
	route, err := repo.FindByPublicIDForUser(ctx, userID, routePublicID)
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
	updated, err := repo.UpdateFeedback(ctx, route, event)
	if err != nil {
		return FeedbackResponse{}, err
	}
	return FeedbackResponse{
		PublicID: updated.PublicID,
		Status:   updated.Status,
	}, nil
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
