package imageroute

import (
	"context"

	"hestia/server/internal/common/id"
	"hestia/server/internal/domain/generator"
)

const (
	StatusCandidate = "candidate"
	SourceRule      = "rule_generator"
)

type Repository interface {
	CreateMany(ctx context.Context, items []Route) ([]Route, error)
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

func strategyMap(strategy generator.StrategyBlock) map[string]any {
	return map[string]any{
		"direction":    strategy.Direction,
		"steps":        strategy.Steps,
		"alternatives": strategy.Alternatives,
	}
}
