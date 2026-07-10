package agent

import (
	"strings"
	"testing"
)

func TestADKRunnerQueryUsesLightDraftSummary(t *testing.T) {
	query := adkRunnerQuery(AdviceRunInput{
		Text: "鞋子换舒服点",
		RecentMessages: []ChatMessage{{
			Role:        ChatRoleUser,
			ContentText: "明天见客户",
		}},
		CurrentDraft: &Draft{
			PublicID:          "drf_test",
			CurrentRevisionNo: 3,
			SceneLabel:        "见客户",
			Sections: []DraftSection{{
				SectionType: SectionTypeOutfit,
				ContentJSON: map[string]any{
					"title":            "清爽通勤",
					"summary":          "米白衬衫搭直筒裤",
					"why_text":         "这段完整理由不应默认进入 prompt",
					"avoid_text":       "避免",
					"alternative_text": "替代",
				},
			}},
		},
	})
	if !containsAll(query, []string{"最近聊天", "明天见客户", "drf_test", "清爽通勤", "鞋子换舒服点"}) {
		t.Fatalf("expected query to include light context, got %s", query)
	}
	if containsAll(query, []string{"这段完整理由不应默认进入 prompt"}) {
		t.Fatalf("expected query to omit full draft body, got %s", query)
	}
}

func TestParseAdviceRunOutputJSONMapsToolCalls(t *testing.T) {
	output, err := parseAdviceRunOutputJSON("```json\n" + `{
  "assistant_text": "已更新穿搭。",
  "decision_label": "update_outfit",
  "tool_calls": [
    {
      "name": "update_advice_draft",
      "tool_call_id": "call_update_1",
      "input_summary": "鞋子换稳一点",
      "update_draft_input": {
        "public_id": "drf_test",
        "user_intent": "鞋子换稳一点",
        "revision_summary": "更新鞋子",
        "sections": [
          {
            "section_type": "outfit",
            "content_schema_version": "v1",
            "revision_summary": "更新穿搭",
            "content_json": {
              "title": "稳一点的通勤穿搭",
              "summary": "鞋子换成低跟乐福鞋"
            }
          }
        ]
      }
    }
  ]
}` + "\n```")
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if output.AssistantText != "已更新穿搭。" || output.DecisionLabel != "update_outfit" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if len(output.ToolCalls) != 1 || output.ToolCalls[0].UpdateDraftInput == nil {
		t.Fatalf("expected update tool call, got %#v", output.ToolCalls)
	}
	if output.ToolCalls[0].ToolCallID != "call_update_1" {
		t.Fatalf("expected tool call id, got %#v", output.ToolCalls[0])
	}
	update := output.ToolCalls[0].UpdateDraftInput
	if update.PublicID != "drf_test" || update.Sections[0].SectionType != SectionTypeOutfit {
		t.Fatalf("unexpected update input: %#v", update)
	}
	if update.Sections[0].ContentJSON["title"] != "稳一点的通勤穿搭" {
		t.Fatalf("unexpected section content: %#v", update.Sections[0].ContentJSON)
	}
}

func containsAll(text string, values []string) bool {
	for _, value := range values {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}

func TestParseAdviceRunOutputJSONMapsCreateTargetDate(t *testing.T) {
	output, err := parseAdviceRunOutputJSON(`{
  "assistant_text": "已创建明天的建议。",
  "decision_label": "create_draft",
  "tool_calls": [
    {
      "name": "create_advice_draft",
      "create_draft_input": {
        "target_date": "2026-07-11",
        "scene_label": "明天见客户",
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
      }
    }
  ]
}`)
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if len(output.ToolCalls) != 1 || output.ToolCalls[0].CreateDraftInput == nil {
		t.Fatalf("expected create tool call, got %#v", output.ToolCalls)
	}
	targetDate := output.ToolCalls[0].CreateDraftInput.TargetDate
	if targetDate == nil || targetDate.Format("2006-01-02") != "2026-07-11" {
		t.Fatalf("expected parsed target date, got %#v", targetDate)
	}
}
