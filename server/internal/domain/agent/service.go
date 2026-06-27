package agent

import (
	"context"
	"strings"

	"hestia/server/internal/domain/wardrobe"
)

const defaultStreamStatusText = "agent stream ready"

type WardrobeAdviceService interface {
	AdviceContextItems(ctx context.Context, userID int64, filter wardrobe.AdviceContextFilter) ([]wardrobe.Item, error)
}

type Service struct {
	wardrobe WardrobeAdviceService
}

func NewService() *Service {
	return &Service{}
}

func NewServiceWithWardrobe(wardrobeService WardrobeAdviceService) *Service {
	return &Service{wardrobe: wardrobeService}
}

func (s *Service) StreamStatus(ctx context.Context, userID int64) StreamStatus {
	text := defaultStreamStatusText
	if s == nil || s.wardrobe == nil || userID == 0 {
		return StreamStatus{Text: text}
	}
	items, err := s.wardrobe.AdviceContextItems(ctx, userID, wardrobe.AdviceContextFilter{Limit: 5})
	if err != nil || len(items) == 0 {
		return StreamStatus{Text: text}
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return StreamStatus{Text: text}
	}
	return StreamStatus{Text: text + "；核心衣橱：" + strings.Join(names, "、")}
}
