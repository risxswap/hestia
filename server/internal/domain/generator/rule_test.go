package generator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRuleReportGeneratorBuildsActionableInitialReport(t *testing.T) {
	result, err := NewRuleReportGenerator().GenerateInitialReport(context.Background(), InitialReportInput{
		StyleGoals: []string{"干净利落", "通勤有气质"},
		Avoidances: []string{"过度甜美", "显拖沓"},
		Scenarios:  []string{"工作日通勤", "周末约会"},
		WardrobeItems: []WardrobeItemInput{
			{Name: "米白衬衫", Category: "上装", Color: "米白", Silhouette: "微宽松"},
			{Name: "直筒牛仔裤", Category: "下装", Color: "蓝色", Silhouette: "直筒"},
		},
		ReferenceStyles: []string{"刘诗诗"},
	})
	if err != nil {
		t.Fatalf("GenerateInitialReport returned error: %v", err)
	}

	if len(result.Routes) < 1 {
		t.Fatalf("expected at least one route, got %d", len(result.Routes))
	}
	if len(result.Routes) > 3 {
		t.Fatalf("expected at most three routes, got %d", len(result.Routes))
	}
	if len(result.ActionItems) == 0 {
		t.Fatalf("expected action items")
	}
	for _, gap := range result.WardrobeGaps {
		raw, err := json.Marshal(gap)
		if err != nil {
			t.Fatalf("marshal gap: %v", err)
		}
		lower := strings.ToLower(string(raw))
		if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "url") || strings.Contains(lower, "link") {
			t.Fatalf("wardrobe gap must not contain product link fields or URLs: %s", raw)
		}
	}
	if !strings.Contains(result.ReferenceStyleLogic, "参考造型逻辑") {
		t.Fatalf("expected reference style logic wording, got %q", result.ReferenceStyleLogic)
	}
	if strings.Contains(result.ReferenceStyleLogic, "你像") {
		t.Fatalf("reference style logic must not say user looks like a celebrity: %q", result.ReferenceStyleLogic)
	}
}
