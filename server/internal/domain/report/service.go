package report

import (
	"context"
	"errors"
	"time"

	"hestia/server/internal/common/id"
)

var ErrReportNotFound = errors.New("report not found")

type Repository interface {
	Create(ctx context.Context, item Report) (Report, error)
	AddRoutes(ctx context.Context, reportID int64, routes []ReportRoute) error
	MarkReady(ctx context.Context, reportID int64) error
	LatestInitialForUser(ctx context.Context, userID int64) (Report, error)
	FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Report, error)
	RoutesByReportID(ctx context.Context, reportID int64) ([]ReportRoute, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateInitialReport(ctx context.Context, input CreateInitialInput) (Report, error) {
	now := time.Now().UTC()
	return s.repo.Create(ctx, Report{
		PublicID:        id.NewPublicID("rpt"),
		UserID:          input.UserID,
		ProfileID:       input.ProfileID,
		ReportType:      TypeInitial,
		Status:          StatusGenerating,
		Title:           input.Title,
		Summary:         input.Summary,
		ContentJSON:     input.ContentJSON,
		ContextSnapshot: input.ContextSnapshot,
		StyleRefsJSON:   input.StyleRefsJSON,
		JobID:           input.JobID,
		GeneratedAt:     &now,
	})
}

func (s *Service) AttachRoutes(ctx context.Context, reportID int64, routes []ReportRoute) error {
	if len(routes) == 0 {
		return nil
	}
	return s.repo.AddRoutes(ctx, reportID, routes)
}

func (s *Service) MarkReady(ctx context.Context, reportID int64) error {
	return s.repo.MarkReady(ctx, reportID)
}

func (s *Service) LatestForUser(ctx context.Context, userID int64) (Response, error) {
	item, err := s.repo.LatestInitialForUser(ctx, userID)
	if err != nil {
		return Response{}, err
	}
	return s.response(ctx, item)
}

func (s *Service) GetForUser(ctx context.Context, userID int64, publicID string) (Response, error) {
	item, err := s.repo.FindByPublicIDForUser(ctx, userID, publicID)
	if err != nil {
		return Response{}, err
	}
	return s.response(ctx, item)
}

func (s *Service) response(ctx context.Context, item Report) (Response, error) {
	routes, err := s.repo.RoutesByReportID(ctx, item.ID)
	if err != nil {
		return Response{}, err
	}
	routeSnapshots := make([]any, 0, len(routes))
	for _, route := range routes {
		routeSnapshots = append(routeSnapshots, route.RouteSnapshot)
	}
	return Response{
		PublicID:    item.PublicID,
		Title:       item.Title,
		Summary:     item.Summary,
		ContentJSON: item.ContentJSON,
		Routes:      routeSnapshots,
	}, nil
}
