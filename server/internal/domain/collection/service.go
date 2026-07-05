package collection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/hair"
	"hestia/server/internal/domain/makeup"
)

type ClothesLister interface {
	ListItems(ctx context.Context, userID int64, filter clothes.ListFilter) ([]clothes.Item, error)
}

type Service struct {
	clothes ClothesLister
	hair    HairLister
	makeup  MakeupLister
}

type HairLister interface {
	ListItems(ctx context.Context, userID int64, filter hair.ListFilter) ([]hair.Item, error)
}

type MakeupLister interface {
	ListItems(ctx context.Context, userID int64, filter makeup.ListFilter) ([]makeup.Item, error)
}

func NewService(clothesLister ClothesLister, hairLister ...HairLister) *Service {
	service := &Service{clothes: clothesLister}
	if len(hairLister) > 0 {
		service.hair = hairLister[0]
	}
	return service
}

func NewServiceWithDomains(clothesLister ClothesLister, hairLister HairLister, makeupLister MakeupLister) *Service {
	return &Service{clothes: clothesLister, hair: hairLister, makeup: makeupLister}
}

func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	clothesItems := []clothes.Item{}
	if s != nil && s.clothes != nil {
		items, err := s.clothes.ListItems(ctx, userID, clothes.ListFilter{})
		if err != nil {
			return Summary{}, err
		}
		clothesItems = items
	}
	hairItems := []hair.Item{}
	if s != nil && s.hair != nil {
		items, err := s.hair.ListItems(ctx, userID, hair.ListFilter{})
		if err != nil {
			return Summary{}, err
		}
		hairItems = items
	}
	makeupItems := []makeup.Item{}
	if s != nil && s.makeup != nil {
		items, err := s.makeup.ListItems(ctx, userID, makeup.ListFilter{})
		if err != nil {
			return Summary{}, err
		}
		makeupItems = items
	}

	return Summary{
		Types: []TypeSummary{
			{Type: "clothes", Label: "衣服", Count: len(clothesItems), Hint: "常穿单品", Enabled: true, EntryPath: "/pages/clothes/list"},
			{Type: "hair", Label: "发型", Count: len(hairItems), Hint: "常用发型", Enabled: true, EntryPath: "/pages/hair/list"},
			{Type: "makeup", Label: "妆容", Count: len(makeupItems), Hint: "妆容方向", Enabled: true, EntryPath: "/pages/makeup/list"},
		},
		RecentItems: recentItems(clothesItems, hairItems, makeupItems, 6),
	}, nil
}

func recentItems(clothesItems []clothes.Item, hairItems []hair.Item, makeupItems []makeup.Item, limit int) []RecentItem {
	source := make([]recentSource, 0, len(clothesItems)+len(hairItems)+len(makeupItems))
	for _, item := range clothesItems {
		recent := RecentItem{
			Type:      "clothes",
			PublicID:  item.PublicID,
			Title:     item.Name,
			Subtitle:  strings.Trim(strings.Join([]string{item.Category, item.Color}, " · "), " ·"),
			EntryPath: fmt.Sprintf("/pages/clothes/detail?public_id=%s", item.PublicID),
		}
		if item.PrimaryImage != nil && item.PrimaryImage.PreviewURL != "" {
			recent.Image = &RecentImage{PreviewURL: item.PrimaryImage.PreviewURL}
		}
		source = append(source, recentSource{item: recent, updatedAt: item.UpdatedAt})
	}
	for _, item := range hairItems {
		recent := RecentItem{
			Type:      "hair",
			PublicID:  item.PublicID,
			Title:     item.Name,
			Subtitle:  strings.Trim(strings.Join([]string{item.Length, item.Color}, " · "), " ·"),
			EntryPath: fmt.Sprintf("/pages/hair/detail?public_id=%s", item.PublicID),
		}
		if item.PrimaryImage != nil && item.PrimaryImage.PreviewURL != "" {
			recent.Image = &RecentImage{PreviewURL: item.PrimaryImage.PreviewURL}
		}
		source = append(source, recentSource{item: recent, updatedAt: item.UpdatedAt})
	}
	for _, item := range makeupItems {
		recent := RecentItem{
			Type:      "makeup",
			PublicID:  item.PublicID,
			Title:     item.Name,
			Subtitle:  strings.Trim(strings.Join([]string{item.MakeupType, item.Finish}, " · "), " ·"),
			EntryPath: fmt.Sprintf("/pages/makeup/detail?public_id=%s", item.PublicID),
		}
		if item.PrimaryImage != nil && item.PrimaryImage.PreviewURL != "" {
			recent.Image = &RecentImage{PreviewURL: item.PrimaryImage.PreviewURL}
		}
		source = append(source, recentSource{item: recent, updatedAt: item.UpdatedAt})
	}
	sort.SliceStable(source, func(i, j int) bool {
		return source[i].updatedAt.After(source[j].updatedAt)
	})
	max := limit
	if max <= 0 || max > len(source) {
		max = len(source)
	}
	result := make([]RecentItem, 0, max)
	for _, item := range source[:max] {
		result = append(result, item.item)
	}
	return result
}

type recentSource struct {
	item      RecentItem
	updatedAt time.Time
}
