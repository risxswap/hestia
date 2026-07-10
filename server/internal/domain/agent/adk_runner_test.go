package agent

import "testing"

func TestParseAdviceRunOutputJSONMapsToolCalls(t *testing.T) {
	output, err := parseAdviceRunOutputJSON("```json\n" + `{
  "assistant_text": "已更新穿搭。",
  "decision_label": "update_outfit",
  "tool_calls": [
    {
      "name": "update_advice_draft",
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
	update := output.ToolCalls[0].UpdateDraftInput
	if update.PublicID != "drf_test" || update.Sections[0].SectionType != SectionTypeOutfit {
		t.Fatalf("unexpected update input: %#v", update)
	}
	if update.Sections[0].ContentJSON["title"] != "稳一点的通勤穿搭" {
		t.Fatalf("unexpected section content: %#v", update.Sections[0].ContentJSON)
	}
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
