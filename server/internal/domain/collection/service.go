package collection

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"hestia/server/internal/domain/wardrobe"
)

type WardrobeLister interface {
	ListItems(ctx context.Context, userID int64, filter wardrobe.ListFilter) ([]wardrobe.Item, error)
}

type Service struct {
	wardrobe WardrobeLister
}

func NewService(wardrobeLister WardrobeLister) *Service {
	return &Service{wardrobe: wardrobeLister}
}

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	wardrobeItems := []wardrobe.Item{}
	if s != nil && s.wardrobe != nil {
		items, err := s.wardrobe.ListItems(ctx, userID, wardrobe.ListFilter{})
		if err != nil {
			return Summary{}, err
		}
		wardrobeItems = items
	}

	return Summary{
		Types: []TypeSummary{
			{Type: "wardrobe", Label: "衣服", Count: len(wardrobeItems), Hint: "常穿单品", Enabled: true, EntryPath: "/pages/wardrobe/wardrobe"},
			{Type: "hair", Label: "发型", Count: 0, Hint: "常用发型", Enabled: true, EntryPath: "/pages/hair/hair"},
			{Type: "makeup", Label: "妆容", Count: 0, Hint: "妆容方向", Enabled: true, EntryPath: "/pages/makeup/makeup"},
			{Type: "references", Label: "参考", Count: 0, Hint: "参考图", Enabled: true, EntryPath: "/pages/references/references"},
		},
		RecentItems: recentWardrobeItems(wardrobeItems, 6),
	}, nil
}

func recentWardrobeItems(items []wardrobe.Item, limit int) []RecentItem {
	source := append([]wardrobe.Item{}, items...)
	sort.SliceStable(source, func(i, j int) bool {
		return source[i].UpdatedAt.After(source[j].UpdatedAt)
	})
	max := limit
	if max <= 0 || max > len(source) {
		max = len(source)
	}
	result := make([]RecentItem, 0, max)
	for _, item := range source[:max] {
		recent := RecentItem{
			Type:      "wardrobe",
			PublicID:  item.PublicID,
			Title:     item.Name,
			Subtitle:  strings.Trim(strings.Join([]string{item.Category, item.Color}, " · "), " ·"),
			EntryPath: fmt.Sprintf("/pages/wardrobe-detail/wardrobe-detail?public_id=%s", item.PublicID),
		}
		if item.PrimaryImage != nil && item.PrimaryImage.PreviewURL != "" {
			recent.Image = &RecentImage{PreviewURL: item.PrimaryImage.PreviewURL}
		}
		result = append(result, recent)
	}
	return result
}
