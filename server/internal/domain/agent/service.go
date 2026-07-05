package agent

import (
	"context"
	"strings"

	"hestia/server/internal/domain/clothes"
)

const defaultStreamStatusText = "agent stream ready"

type ClothesAdviceService interface {
	AdviceContextItems(ctx context.Context, userID int64, filter clothes.AdviceContextFilter) ([]clothes.Item, error)
}

type Service struct {
	clothes ClothesAdviceService
}

func NewService() *Service {
	return &Service{}
}

func NewServiceWithClothes(clothesService ClothesAdviceService) *Service {
	return &Service{clothes: clothesService}
}

func (s *Service) StreamStatus(ctx context.Context, userID int64) StreamStatus {
	text := defaultStreamStatusText
	if s == nil || s.clothes == nil || userID == 0 {
		return StreamStatus{Text: text}
	}
	items, err := s.clothes.AdviceContextItems(ctx, userID, clothes.AdviceContextFilter{Limit: 5})
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
	return StreamStatus{Text: text + "；核心衣服：" + strings.Join(names, "、")}
}
