package agent

import (
	"context"
	"encoding/json"
	"testing"

	"hestia/server/internal/domain/clothes"

	"github.com/cloudwego/eino/components/tool"
)

func TestAdviceToolsCreateDraftUsesSessionUserAndSourceMessage(t *testing.T) {
	repo := &spyAgentRepo{}
	createTool := mustAdviceTool(t, repo, AdviceToolCreateDraft)
	ctx := contextWithAdviceToolSession(context.Background(), 12, 101)

	raw, err := createTool.InvokableRun(ctx, `{
  "scene_label": "明天见客户",
  "user_intent": "创建建议",
  "sections": [
    {
      "section_type": "outfit",
      "content_schema_version": "v1",
      "content_json": {
        "title": "清爽通勤",
        "summary": "米白衬衫搭直筒裤"
      }
    }
  ]
}`)
	if err != nil {
		t.Fatalf("invoke create tool: %v", err)
	}
	var output adviceToolDraftOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if !output.OK || output.Draft == nil || output.Draft.DraftPublicID != "drf_test" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if !repo.createdDraft || repo.lastCreate.UserID != 12 || repo.lastCreate.SourceMsgID != 101 {
		t.Fatalf("expected session-bound create input, got %#v", repo.lastCreate)
	}
}

func TestAdviceToolsGetCurrentDraftUsesSessionUser(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	getTool := mustAdviceTool(t, repo, "get_current_advice_draft")
	ctx := contextWithAdviceToolSession(context.Background(), 12, 101)

	raw, err := getTool.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("invoke get current tool: %v", err)
	}
	var output adviceToolDraftOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if !output.OK || output.Draft == nil || output.Draft.DraftPublicID != "drf_test" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestAdviceToolsRejectMissingSession(t *testing.T) {
	repo := &spyAgentRepo{}
	createTool := mustAdviceTool(t, repo, AdviceToolCreateDraft)

	_, err := createTool.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatalf("expected missing session error")
	}
}

func TestAdviceToolsWardrobeContextUsesSessionUser(t *testing.T) {
	repo := &spyAgentRepo{}
	clothesService := &spyClothesAdviceService{items: []clothes.Item{{
		PublicID:             "wdi_shirt",
		Name:                 "米白衬衫",
		Category:             "top",
		Color:                "米白",
		IsCore:               true,
		RecommendationStatus: clothes.RecommendationStatusPreferred,
	}}}
	wardrobeTool := mustAdviceTool(t, repo, "get_wardrobe_context", clothesService)
	ctx := contextWithAdviceToolSession(context.Background(), 12, 101)

	raw, err := wardrobeTool.InvokableRun(ctx, `{"scene":"通勤","limit":3}`)
	if err != nil {
		t.Fatalf("invoke wardrobe tool: %v", err)
	}
	var output adviceToolWardrobeOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if clothesService.userID != 12 || clothesService.filter.Scene != "通勤" || clothesService.filter.Limit != 3 {
		t.Fatalf("expected session-bound wardrobe filter, got user=%d filter=%#v", clothesService.userID, clothesService.filter)
	}
	if !output.OK || len(output.Items) != 1 || output.Items[0].PublicID != "wdi_shirt" {
		t.Fatalf("unexpected wardrobe output: %#v", output)
	}
}

func mustAdviceTool(t *testing.T, repo Repository, name string, clothesServices ...ClothesAdviceService) tool.InvokableTool {
	t.Helper()
	tools, err := NewAdviceTools(repo, clothesServices...)
	if err != nil {
		t.Fatalf("new advice tools: %v", err)
	}
	for _, item := range tools {
		info, err := item.Info(context.Background())
		if err != nil {
			t.Fatalf("tool info: %v", err)
		}
		if info.Name != name {
			continue
		}
		invokable, ok := item.(tool.InvokableTool)
		if !ok {
			t.Fatalf("tool %s is not invokable", name)
		}
		return invokable
	}
	t.Fatalf("tool %s not found", name)
	return nil
}

type spyClothesAdviceService struct {
	userID int64
	filter clothes.AdviceContextFilter
	items  []clothes.Item
}

func (s *spyClothesAdviceService) AdviceContextItems(_ context.Context, userID int64, filter clothes.AdviceContextFilter) ([]clothes.Item, error) {
	s.userID = userID
	s.filter = filter
	return s.items, nil
}
