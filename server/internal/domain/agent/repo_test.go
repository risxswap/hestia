package agent

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestRepositoryCreateDraftCreatesSectionsAndVersions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_drafts")).
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_draft_sections")).
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_draft_section_versions")).
		WillReturnResult(sqlmock.NewResult(30, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_draft_sections")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	draft, err := repo.CreateDraft(context.Background(), CreateDraftInput{
		UserID:     12,
		SceneLabel: "明天见客户",
		UserIntent: "创建建议",
		Sections: []DraftSectionInput{{
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("清爽通勤"),
			RevisionSummary:      "生成穿搭",
		}},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.ID != 10 || draft.CurrentRevisionNo != 1 || len(draft.Sections) != 1 {
		t.Fatalf("unexpected draft: %#v", draft)
	}
	if draft.Sections[0].ID != 20 || draft.Sections[0].CurrentSectionVersionID != 30 {
		t.Fatalf("unexpected section: %#v", draft.Sections[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryCreateChatMessageWritesSourceMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_msgs")).
		WillReturnResult(sqlmock.NewResult(101, 1))

	message, err := repo.CreateChatMessage(context.Background(), CreateChatMessageInput{
		UserID:      12,
		SourceMsgID: 100,
		Role:        ChatRoleAssistant,
		MsgType:     ChatMsgTypeText,
		ContentText: "处理中",
		Status:      ChatStatusGenerating,
	})
	if err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	if message.ID != 101 || message.SourceMsgID != 100 || message.Role != ChatRoleAssistant {
		t.Fatalf("unexpected message: %#v", message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryCreateAgentRunStepWritesObservableStep(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO agent_run_steps")).
		WillReturnResult(sqlmock.NewResult(201, 1))

	err = repo.CreateAgentRunStep(context.Background(), AgentRunStepInput{
		UserID:         12,
		SourceMsgID:    100,
		AssistantMsgID: 101,
		StepNo:         1,
		StepType:       AgentStepTypeModelDecision,
		Status:         AgentStepStatusSucceeded,
		DecisionLabel:  "draft",
	})
	if err != nil {
		t.Fatalf("create run step: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryUpdateDraftOnlyVersionsChangedSections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	expectDraftForUpdate(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, section_type, current_section_version_no FROM advice_draft_sections")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "section_type", "current_section_version_no"}).
			AddRow(20, SectionTypeOutfit, 1).
			AddRow(21, SectionTypeHair, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_draft_section_versions")).
		WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_draft_sections")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_drafts")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectDraftWithSections(mock, 2)
	mock.ExpectCommit()

	draft, err := repo.UpdateDraftSections(context.Background(), UpdateDraftInput{
		UserID:     12,
		PublicID:   "drf_test",
		UserIntent: "鞋子换舒服点",
		Sections: []DraftSectionInput{{
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("换成乐福鞋"),
			RevisionSummary:      "更新鞋子",
		}},
	})
	if err != nil {
		t.Fatalf("update draft: %v", err)
	}
	if draft.CurrentRevisionNo != 2 {
		t.Fatalf("expected revision 2, got %#v", draft)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryConfirmDraftCreatesAdviceSections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	expectDraftForConfirm(mock)
	expectDraftSectionsForConfirm(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advices")).
		WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(52, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_drafts")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	advice, err := repo.ConfirmDraft(context.Background(), 12, "drf_test")
	if err != nil {
		t.Fatalf("confirm draft: %v", err)
	}
	if advice.ID != 40 || advice.PublicID == "" || len(advice.Sections) != 3 {
		t.Fatalf("unexpected advice: %#v", advice)
	}
	if advice.Sections[0].ID != 50 || advice.Sections[0].SourceDraftSectionVersionID != 30 {
		t.Fatalf("unexpected advice section: %#v", advice.Sections[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryConfirmDraftRejectsIncompleteSections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	expectDraftForConfirm(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, draft_id, user_id, section_type, current_section_version_id, current_section_version_no, content_schema_version, content_json, created_at, updated_at FROM advice_draft_sections")).
		WillReturnRows(sqlmock.NewRows(sectionRowColumns()).
			AddRow(20, "ads_outfit", 10, 12, SectionTypeOutfit, 30, 1, "v1", `{"title":"清爽通勤","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil))
	mock.ExpectRollback()

	_, err = repo.ConfirmDraft(context.Background(), 12, "drf_test")
	if !errors.Is(err, ErrDraftIncomplete) {
		t.Fatalf("expected ErrDraftIncomplete, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryConfirmDraftReturnsNotFoundWhenConcurrentConfirmWins(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	expectDraftForConfirm(mock)
	expectDraftSectionsForConfirm(mock)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advices")).
		WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(52, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_drafts")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = repo.ConfirmDraft(context.Background(), 12, "drf_test")
	if !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryConfirmDraftReturnsExistingAdviceWhenAlreadyConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no, confirmed_advice_id, created_at, updated_at FROM advice_drafts")).
		WillReturnRows(sqlmock.NewRows(draftRowColumns()).
			AddRow(10, "drf_test", 12, nil, DraftStatusConfirmed, "", "明天见客户", nil, "", "", "", "", "", 2, 40, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, source_draft_id, source_draft_revision_no, status, scene_key, scene_label, occasion, weather_text, mood_text, style_goal, avoid_goal, created_at, updated_at FROM advices")).
		WillReturnRows(sqlmock.NewRows(adviceRowColumns()).
			AddRow(40, "adv_existing", 12, nil, 10, 2, AdviceStatusReady, "", "明天见客户", "", "", "", "", "", nil, nil))
	mock.ExpectCommit()

	advice, err := repo.ConfirmDraft(context.Background(), 12, "drf_test")
	if err != nil {
		t.Fatalf("confirm existing draft: %v", err)
	}
	if advice.PublicID != "adv_existing" {
		t.Fatalf("expected existing advice public id, got %#v", advice)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryDiscardDraftReturnsNotFoundWhenNoRowsAffected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_drafts")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.DiscardDraft(context.Background(), 12, "drf_missing")
	if !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func expectDraftForUpdate(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no, confirmed_advice_id, created_at, updated_at FROM advice_drafts")).
		WillReturnRows(sqlmock.NewRows(draftRowColumns()).
			AddRow(10, "drf_test", 12, nil, DraftStatusDraft, "", "明天见客户", nil, "", "", "", "", "", 1, nil, nil, nil))
}

func expectDraftForConfirm(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no, confirmed_advice_id, created_at, updated_at FROM advice_drafts")).
		WillReturnRows(sqlmock.NewRows(draftRowColumns()).
			AddRow(10, "drf_test", 12, nil, DraftStatusDraft, "", "明天见客户", nil, "", "", "", "", "", 2, nil, nil, nil))
}

func expectDraftWithSections(mock sqlmock.Sqlmock, revision int) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no, confirmed_advice_id, created_at, updated_at FROM advice_drafts")).
		WillReturnRows(sqlmock.NewRows(draftRowColumns()).
			AddRow(10, "drf_test", 12, nil, DraftStatusDraft, "", "明天见客户", nil, "", "", "", "", "", revision, nil, nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, draft_id, user_id, section_type, current_section_version_id, current_section_version_no, content_schema_version, content_json, created_at, updated_at FROM advice_draft_sections")).
		WillReturnRows(sqlmock.NewRows(sectionRowColumns()).
			AddRow(20, "ads_outfit", 10, 12, SectionTypeOutfit, 31, 2, "v1", `{"title":"换成乐福鞋","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil))
}

func expectDraftSectionsForConfirm(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, draft_id, user_id, section_type, current_section_version_id, current_section_version_no, content_schema_version, content_json, created_at, updated_at FROM advice_draft_sections")).
		WillReturnRows(sqlmock.NewRows(sectionRowColumns()).
			AddRow(20, "ads_outfit", 10, 12, SectionTypeOutfit, 30, 1, "v1", `{"title":"清爽通勤","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil).
			AddRow(21, "ads_hair", 10, 12, SectionTypeHair, 31, 1, "v1", `{"title":"低丸子头","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil).
			AddRow(22, "ads_makeup", 10, 12, SectionTypeMakeup, 32, 1, "v1", `{"title":"清透妆","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil))
}

func draftRowColumns() []string {
	return []string{"id", "public_id", "user_id", "source_msg_id", "status", "scene_key", "scene_label", "target_date", "occasion", "weather_text", "mood_text", "style_goal", "avoid_goal", "current_revision_no", "confirmed_advice_id", "created_at", "updated_at"}
}

func sectionRowColumns() []string {
	return []string{"id", "public_id", "draft_id", "user_id", "section_type", "current_section_version_id", "current_section_version_no", "content_schema_version", "content_json", "created_at", "updated_at"}
}

func adviceRowColumns() []string {
	return []string{"id", "public_id", "user_id", "source_msg_id", "source_draft_id", "source_draft_revision_no", "status", "scene_key", "scene_label", "occasion", "weather_text", "mood_text", "style_goal", "avoid_goal", "created_at", "updated_at"}
}

func requiredAdviceJSON(title string) map[string]any {
	return map[string]any{
		"title":            title,
		"summary":          "摘要",
		"why_text":         "适合",
		"avoid_text":       "避免",
		"alternative_text": "替代",
	}
}
