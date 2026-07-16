package agent

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

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
	for i := int64(0); i < 3; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_draft_sections")).
			WillReturnResult(sqlmock.NewResult(20+i, 1))
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_draft_section_versions")).
			WillReturnResult(sqlmock.NewResult(30+i, 1))
		mock.ExpectExec(regexp.QuoteMeta("UPDATE advice_draft_sections")).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	draft, err := repo.CreateDraft(context.Background(), CreateDraftInput{
		UserID:     12,
		SceneLabel: "明天见客户",
		UserIntent: "创建建议",
		Sections:   validThreeSectionInputs(),
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.ID != 10 || draft.CurrentRevisionNo != 1 || len(draft.Sections) != 3 {
		t.Fatalf("unexpected draft: %#v", draft)
	}
	if draft.Sections[0].ID != 20 || draft.Sections[0].CurrentSectionVersionID != 30 {
		t.Fatalf("unexpected section: %#v", draft.Sections[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryCreateDraftRejectsIncompleteSections(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	_, err = repo.CreateDraft(context.Background(), CreateDraftInput{
		UserID: 12,
		Sections: []DraftSectionInput{{
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("清爽通勤"),
		}},
	})
	if !errors.Is(err, ErrDraftInvalid) {
		t.Fatalf("expected ErrDraftInvalid, got %v", err)
	}
}

func TestRepositoryUpdateDraftRejectsUnknownSectionContent(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	_, err = repo.UpdateDraftSections(context.Background(), UpdateDraftInput{
		UserID:   12,
		PublicID: "drf_test",
		Sections: []DraftSectionInput{{
			SectionType: SectionTypeOutfit,
			ContentJSON: map[string]any{
				"title":            "清爽通勤",
				"summary":          "摘要",
				"why_text":         "适合",
				"avoid_text":       "避免",
				"alternative_text": "替代",
				"raw_prompt":       "不能入库",
			},
		}},
	})
	if !errors.Is(err, ErrDraftInvalid) {
		t.Fatalf("expected ErrDraftInvalid, got %v", err)
	}
}

func TestRepositoryCreateChatMessageWritesSourceMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	createdAt := time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_msgs")).
		WithArgs(sqlmock.AnyArg(), int64(12), int64(100), ChatRoleAssistant, ChatMsgTypeText, "处理中", sqlmock.AnyArg(), "", int64(0), "", ChatStatusGenerating, createdAt).
		WillReturnResult(sqlmock.NewResult(101, 1))

	message, err := repo.CreateChatMessage(context.Background(), CreateChatMessageInput{
		UserID:      12,
		SourceMsgID: 100,
		Role:        ChatRoleAssistant,
		MsgType:     ChatMsgTypeText,
		ContentText: "处理中",
		AssetRefs: []ChatAssetRef{{
			AssetPublicID: "ast_photo",
			AssetType:     "chat_image",
			Note:          "用户上传图",
		}},
		Status:    ChatStatusGenerating,
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	if message.ID != 101 || message.SourceMsgID != 100 || message.Role != ChatRoleAssistant || !message.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected message: %#v", message)
	}
	if len(message.AssetRefs) != 1 || message.AssetRefs[0].AssetPublicID != "ast_photo" {
		t.Fatalf("expected asset refs on created message, got %#v", message.AssetRefs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryListRecentChatMessagesReturnsChronologicalSentMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, role, msg_type, content_text, asset_refs, related_type, related_id, related_public_id, status FROM chat_msgs")).
		WithArgs(int64(12), ChatStatusSent, 12).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "user_id", "source_msg_id", "role", "msg_type", "content_text", "asset_refs", "related_type", "related_id", "related_public_id", "status"}).
			AddRow(3, "msg_assistant", 12, 2, ChatRoleAssistant, ChatMsgTypeDraftCard, "已更新草稿", nil, "advice_draft", 10, "drf_test", ChatStatusSent).
			AddRow(2, "msg_user", 12, nil, ChatRoleUser, ChatMsgTypeText, "鞋子换舒服点", `[{"asset_public_id":"ast_photo","asset_type":"chat_image","note":"用户上传图"}]`, nil, nil, nil, ChatStatusSent))

	messages, err := repo.ListRecentChatMessages(context.Background(), 12, 12)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 || messages[0].PublicID != "msg_user" || messages[1].PublicID != "msg_assistant" {
		t.Fatalf("expected chronological messages, got %#v", messages)
	}
	if messages[1].RelatedPublicID != "drf_test" {
		t.Fatalf("expected related draft public id, got %#v", messages[1])
	}
	if len(messages[0].AssetRefs) != 1 || messages[0].AssetRefs[0].AssetPublicID != "ast_photo" {
		t.Fatalf("expected asset refs on recent user message, got %#v", messages[0].AssetRefs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepositoryListChatMessagesIncludesGeneratingMessagesWithCreatedAt(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))
	startedAt := time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, public_id, user_id, source_msg_id, role, msg_type, content_text, asset_refs, related_type, related_id, related_public_id, status, created_at, updated_at FROM chat_msgs")).
		WithArgs(int64(12), 50).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "user_id", "source_msg_id", "role", "msg_type", "content_text", "asset_refs", "related_type", "related_id", "related_public_id", "status", "created_at", "updated_at"}).
			AddRow(3, "msg_assistant", 12, 2, ChatRoleAssistant, ChatMsgTypeText, nil, nil, nil, nil, nil, ChatStatusGenerating, startedAt, startedAt).
			AddRow(2, "msg_user", 12, nil, ChatRoleUser, ChatMsgTypeText, "明天见客户", nil, nil, nil, nil, ChatStatusSent, startedAt.Add(-time.Minute), startedAt.Add(-time.Minute)))

	messages, err := repo.ListChatMessages(context.Background(), 12, 50)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(messages) != 2 || messages[0].PublicID != "msg_user" || messages[1].PublicID != "msg_assistant" {
		t.Fatalf("expected chronological visible messages, got %#v", messages)
	}
	if !messages[1].CreatedAt.Equal(startedAt) || messages[1].Status != ChatStatusGenerating {
		t.Fatalf("expected generating assistant message with persisted start time, got %#v", messages[1])
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
	startedAt := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(25 * time.Millisecond)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO agent_run_steps")).
		WithArgs(
			sqlmock.AnyArg(), int64(12), int64(100), int64(101), 1, AgentStepTypeModelDecision, AgentStepStatusSucceeded,
			"agent_chat", "qwen", "qwen-plus", "v1", 8, "create_advice_draft", "call_1", "draft", "用户输入", "模型输出",
			"advice_draft", int64(30), "drf_test", startedAt, finishedAt, 25, "",
		).
		WillReturnResult(sqlmock.NewResult(201, 1))

	err = repo.CreateAgentRunStep(context.Background(), AgentRunStepInput{
		UserID:          12,
		SourceMsgID:     100,
		AssistantMsgID:  101,
		StepNo:          1,
		StepType:        AgentStepTypeModelDecision,
		Status:          AgentStepStatusSucceeded,
		UsageKey:        "agent_chat",
		ProviderCode:    "qwen",
		ModelCode:       "qwen-plus",
		PromptVersion:   "v1",
		MaxIterations:   8,
		ToolName:        "create_advice_draft",
		ToolCallID:      "call_1",
		DecisionLabel:   "draft",
		InputSummary:    "用户输入",
		OutputSummary:   "模型输出",
		RelatedType:     "advice_draft",
		RelatedID:       30,
		RelatedPublicID: "drf_test",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
		DurationMS:      25,
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

func TestRepositoryListDraftVersionsGroupsByDraftRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	expectDraftForConfirm(mock)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT public_id, section_type, section_version_no, draft_revision_no, source_msg_id, user_intent, revision_summary, content_schema_version, content_json, created_at FROM advice_draft_section_versions")).
		WithArgs(int64(10), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "section_type", "section_version_no", "draft_revision_no", "source_msg_id", "user_intent", "revision_summary", "content_schema_version", "content_json", "created_at"}).
			AddRow("adsv_outfit_1", SectionTypeOutfit, 1, 1, 100, "创建建议", "创建穿搭", "v1", `{"title":"清爽通勤","summary":"摘要"}`, nil).
			AddRow("adsv_hair_1", SectionTypeHair, 1, 1, 100, "创建建议", "创建发型", "v1", `{"title":"低丸子头","summary":"摘要"}`, nil).
			AddRow("adsv_outfit_2", SectionTypeOutfit, 2, 2, 101, "鞋子换舒服点", "更新鞋子", "v1", `{"title":"换成乐福鞋","summary":"摘要"}`, nil))

	revisions, err := repo.ListDraftVersions(context.Background(), 12, "drf_test")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("expected two revisions, got %#v", revisions)
	}
	if revisions[0].DraftRevisionNo != 1 || len(revisions[0].Sections) != 2 {
		t.Fatalf("expected first revision to group two sections, got %#v", revisions[0])
	}
	if revisions[1].DraftRevisionNo != 2 || revisions[1].Sections[0].SectionVersionNo != 2 {
		t.Fatalf("unexpected second revision: %#v", revisions[1])
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

func TestRepositoryConfirmDraftSplitsOutfitItemsIntoRefs(t *testing.T) {
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
			AddRow(20, "ads_outfit", 10, 12, SectionTypeOutfit, 30, 1, "v1", outfitAdviceWithRefsJSON(), nil, nil).
			AddRow(21, "ads_hair", 10, 12, SectionTypeHair, 31, 1, "v1", `{"title":"低丸子头","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil).
			AddRow(22, "ads_makeup", 10, 12, SectionTypeMakeup, 32, 1, "v1", `{"title":"清透妆","summary":"摘要","why_text":"适合","avoid_text":"避免","alternative_text":"替代"}`, nil, nil))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advices")).
		WillReturnResult(sqlmock.NewResult(40, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_sections")).
		WillReturnResult(sqlmock.NewResult(50, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM clothes")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(70))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_clothes_refs")).
		WillReturnResult(sqlmock.NewResult(80, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO advice_gap_refs")).
		WillReturnResult(sqlmock.NewResult(81, 1))
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
	if len(advice.Sections) != 3 {
		t.Fatalf("expected three sections, got %#v", advice)
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

func validThreeSectionInputs() []DraftSectionInput {
	return []DraftSectionInput{
		{
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("清爽通勤"),
			RevisionSummary:      "生成穿搭",
		},
		{
			SectionType:          SectionTypeHair,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("低丸子头"),
			RevisionSummary:      "生成发型",
		},
		{
			SectionType:          SectionTypeMakeup,
			ContentSchemaVersion: "v1",
			ContentJSON:          requiredAdviceJSON("清透妆"),
			RevisionSummary:      "生成妆容",
		},
	}
}

func outfitAdviceWithRefsJSON() string {
	return `{
		"title":"清爽通勤",
		"summary":"摘要",
		"why_text":"适合",
		"avoid_text":"避免",
		"alternative_text":"替代",
		"items":[
			{"role":"top","text":"米白衬衫","source_type":"wardrobe_item","source_public_id":"wdi_shirt","reason_text":"清爽"},
			{"role":"shoe","text":"低跟乐福鞋","source_type":"gap_item","source_public_id":"","reason_text":"更稳定"}
		]
	}`
}
