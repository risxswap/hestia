package agent

import (
	"context"
	"encoding/json"
	"testing"

	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/memory"
	"hestia/server/internal/domain/profile"

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

func TestAdviceToolsProfileContextUsesSessionUser(t *testing.T) {
	repo := &spyAgentRepo{}
	profileService := &spyProfileContextService{summary: profile.Summary{
		User: profile.UserSummary{Nickname: "小禾", OnboardingStatus: "completed"},
		Profile: &profile.ProfileSummary{
			ProfilePublicID:    "prf_test",
			Gender:             "female",
			BodyNotes:          "肩线偏窄",
			LifestyleScenarios: []string{"通勤", "约会"},
			StyleGoalSummary:   "清爽、显高",
		},
		Preferences: profile.PreferencesSummary{
			StyleGoals: []string{"清爽通勤"},
			Avoidances: []string{"过度甜美"},
		},
	}}
	profileTool := mustAdviceToolWithDependencies(t, repo, "get_profile_context", AdviceToolDependencies{Profile: profileService})
	ctx := contextWithAdviceToolSession(context.Background(), 12, 101)

	raw, err := profileTool.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatalf("invoke profile tool: %v", err)
	}
	var output adviceToolProfileOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if profileService.userID != 12 {
		t.Fatalf("expected session-bound profile lookup, got user=%d", profileService.userID)
	}
	if !output.OK || output.Profile == nil || output.Profile.StyleGoalSummary != "清爽、显高" {
		t.Fatalf("unexpected profile output: %#v", output)
	}
	if len(output.Preferences.Avoidances) != 1 || output.Preferences.Avoidances[0] != "过度甜美" {
		t.Fatalf("expected preference context, got %#v", output.Preferences)
	}
}

func TestAdviceToolsMemoryContextUsesSessionUserAndLimit(t *testing.T) {
	repo := &spyAgentRepo{}
	memoryService := &spyMemoryContextService{items: []memory.Item{
		{
			PublicID:    "mem_fact",
			MemoryType:  memory.TypeFact,
			MemoryKey:   "height",
			MemoryValue: "165cm",
			DisplayText: "身高 165cm",
			Polarity:    memory.PolarityNeutral,
		},
		{
			PublicID:    "mem_avoid",
			MemoryType:  memory.TypeAvoidance,
			MemoryKey:   "avoid_style",
			MemoryValue: "不喜欢夸张泡泡袖",
			DisplayText: "不喜欢夸张泡泡袖",
			Polarity:    memory.PolarityNegative,
		},
	}}
	memoryTool := mustAdviceToolWithDependencies(t, repo, "get_memory_context", AdviceToolDependencies{Memory: memoryService})
	ctx := contextWithAdviceToolSession(context.Background(), 12, 101)

	raw, err := memoryTool.InvokableRun(ctx, `{"query":"通勤穿搭","limit":1}`)
	if err != nil {
		t.Fatalf("invoke memory tool: %v", err)
	}
	var output adviceToolMemoryOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		t.Fatalf("decode output: %v raw=%s", err, raw)
	}
	if memoryService.userID != 12 || memoryService.query != "通勤穿搭" || memoryService.limit != 1 {
		t.Fatalf("expected session-bound memory lookup, got user=%d query=%q limit=%d", memoryService.userID, memoryService.query, memoryService.limit)
	}
	if !output.OK || len(output.Items) != 1 || output.Items[0].PublicID != "mem_fact" {
		t.Fatalf("unexpected memory output: %#v", output)
	}
}

func mustAdviceTool(t *testing.T, repo Repository, name string, clothesServices ...ClothesAdviceService) tool.InvokableTool {
	t.Helper()
	tools, err := NewAdviceTools(repo, clothesServices...)
	if err != nil {
		t.Fatalf("new advice tools: %v", err)
	}
	return findAdviceTool(t, tools, name)
}

func mustAdviceToolWithDependencies(t *testing.T, repo Repository, name string, deps AdviceToolDependencies) tool.InvokableTool {
	t.Helper()
	tools, err := NewAdviceToolsWithDependencies(repo, deps)
	if err != nil {
		t.Fatalf("new advice tools: %v", err)
	}
	return findAdviceTool(t, tools, name)
}

func findAdviceTool(t *testing.T, tools []tool.BaseTool, name string) tool.InvokableTool {
	t.Helper()
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

type spyProfileContextService struct {
	userID  int64
	summary profile.Summary
}

func (s *spyProfileContextService) Summary(_ context.Context, userID int64) (profile.Summary, error) {
	s.userID = userID
	return s.summary, nil
}

type spyMemoryContextService struct {
	userID int64
	query  string
	limit  int
	items  []memory.Item
}

func (s *spyMemoryContextService) AgentMemoryContext(_ context.Context, userID int64, query string, limit int) ([]memory.Item, error) {
	s.userID = userID
	s.query = query
	s.limit = limit
	if limit > 0 && limit < len(s.items) {
		return s.items[:limit], nil
	}
	return s.items, nil
}
