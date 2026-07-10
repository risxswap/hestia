package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"hestia/server/internal/common/dbutil"
	"hestia/server/internal/common/id"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	ext sqlx.ExtContext
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return NewMySQLRepositoryWithExt(db)
}

func NewMySQLRepositoryWithExt(ext sqlx.ExtContext) *MySQLRepository {
	return &MySQLRepository{ext: ext}
}

func (r *MySQLRepository) CreateChatMessage(ctx context.Context, input CreateChatMessageInput) (ChatMessage, error) {
	if r == nil || r.ext == nil {
		return ChatMessage{}, ErrRepositoryUnsupported
	}
	publicID := id.NewPublicID("msg")
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO chat_msgs
  (public_id, user_id, source_msg_id, role, msg_type, content_text, related_type, related_id, related_public_id, status)
VALUES
  (?, ?, NULLIF(?, 0), ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, 0), NULLIF(?, ''), ?)
`, publicID, input.UserID, input.SourceMsgID, input.Role, input.MsgType, input.ContentText, input.RelatedType, input.RelatedID, input.RelatedPublicID, input.Status)
	if err != nil {
		return ChatMessage{}, err
	}
	messageID, err := dbutil.RequireLastInsertID(result, "chat message create")
	if err != nil {
		return ChatMessage{}, err
	}
	return ChatMessage{
		ID:              messageID,
		PublicID:        publicID,
		UserID:          input.UserID,
		SourceMsgID:     input.SourceMsgID,
		Role:            input.Role,
		MsgType:         input.MsgType,
		ContentText:     input.ContentText,
		RelatedType:     input.RelatedType,
		RelatedID:       input.RelatedID,
		RelatedPublicID: input.RelatedPublicID,
		Status:          input.Status,
	}, nil
}

func (r *MySQLRepository) UpdateChatMessage(ctx context.Context, input UpdateChatMessageInput) (ChatMessage, error) {
	if r == nil || r.ext == nil {
		return ChatMessage{}, ErrRepositoryUnsupported
	}
	if err := requireAffected(r.ext.ExecContext(ctx, `
UPDATE chat_msgs
SET status = ?,
    msg_type = ?,
    content_text = NULLIF(?, ''),
    related_type = NULLIF(?, ''),
    related_id = NULLIF(?, 0),
    related_public_id = NULLIF(?, ''),
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
`, input.Status, input.MsgType, input.ContentText, input.RelatedType, input.RelatedID, input.RelatedPublicID, time.Now().UTC(), input.ID)); err != nil {
		return ChatMessage{}, err
	}
	return ChatMessage{
		ID:              input.ID,
		Status:          input.Status,
		MsgType:         input.MsgType,
		ContentText:     input.ContentText,
		RelatedType:     input.RelatedType,
		RelatedID:       input.RelatedID,
		RelatedPublicID: input.RelatedPublicID,
	}, nil
}

func (r *MySQLRepository) ListRecentChatMessages(ctx context.Context, userID int64, limit int) ([]ChatMessage, error) {
	if r == nil || r.ext == nil {
		return nil, ErrRepositoryUnsupported
	}
	if limit <= 0 || limit > 20 {
		limit = 12
	}
	var rows []chatMessageRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT id, public_id, user_id, source_msg_id, role, msg_type, content_text, related_type, related_id, related_public_id, status
FROM chat_msgs
WHERE user_id = ?
  AND status = ?
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT ?
`, userID, ChatStatusSent, limit); err != nil {
		return nil, err
	}
	messages := make([]ChatMessage, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		messages = append(messages, rows[i].message())
	}
	return messages, nil
}

func (r *MySQLRepository) CreateAgentRunStep(ctx context.Context, input AgentRunStepInput) error {
	if r == nil || r.ext == nil {
		return ErrRepositoryUnsupported
	}
	_, err := r.ext.ExecContext(ctx, `
INSERT INTO agent_run_steps
  (public_id, user_id, source_msg_id, assistant_msg_id, step_no, step_type, status, decision_label, input_summary, output_summary, related_type, related_id, related_public_id, started_at, finished_at, error_message)
VALUES
  (?, ?, NULLIF(?, 0), ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, 0), NULLIF(?, ''), ?, ?, NULLIF(?, ''))
`, id.NewPublicID("ars"), input.UserID, input.SourceMsgID, input.AssistantMsgID, input.StepNo, input.StepType, input.Status, input.DecisionLabel, input.InputSummary, input.OutputSummary, input.RelatedType, input.RelatedID, input.RelatedPublicID, time.Now().UTC(), time.Now().UTC(), input.ErrorMessage)
	return err
}

func (r *MySQLRepository) CreateDraft(ctx context.Context, input CreateDraftInput) (Draft, error) {
	if r == nil || r.ext == nil {
		return Draft{}, ErrRepositoryUnsupported
	}
	if err := validateCreateDraftInput(input); err != nil {
		return Draft{}, err
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Draft{}, err
		}
		draft, err := (&MySQLRepository{ext: tx}).createDraft(ctx, input)
		if err != nil {
			_ = tx.Rollback()
			return Draft{}, err
		}
		if err := tx.Commit(); err != nil {
			return Draft{}, err
		}
		return draft, nil
	}
	return r.createDraft(ctx, input)
}

func (r *MySQLRepository) GetCurrentDraft(ctx context.Context, userID int64) (Draft, error) {
	if r == nil || r.ext == nil {
		return Draft{}, ErrRepositoryUnsupported
	}
	draft, err := r.findCurrentDraft(ctx, userID)
	if err != nil {
		return Draft{}, err
	}
	sections, err := r.listDraftSections(ctx, draft.ID)
	if err != nil {
		return Draft{}, err
	}
	draft.Sections = sections
	return draft, nil
}

func (r *MySQLRepository) UpdateDraftSections(ctx context.Context, input UpdateDraftInput) (Draft, error) {
	if r == nil || r.ext == nil {
		return Draft{}, ErrRepositoryUnsupported
	}
	if err := validateUpdateDraftInput(input); err != nil {
		return Draft{}, err
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Draft{}, err
		}
		draft, err := (&MySQLRepository{ext: tx}).updateDraftSections(ctx, input)
		if err != nil {
			_ = tx.Rollback()
			return Draft{}, err
		}
		if err := tx.Commit(); err != nil {
			return Draft{}, err
		}
		return draft, nil
	}
	return r.updateDraftSections(ctx, input)
}

func (r *MySQLRepository) ListDraftVersions(ctx context.Context, userID int64, publicID string) ([]DraftRevision, error) {
	if r == nil || r.ext == nil {
		return nil, ErrRepositoryUnsupported
	}
	draft, err := r.findDraftForUser(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	var rows []draftSectionVersionRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT public_id, section_type, section_version_no, draft_revision_no, source_msg_id, user_intent, revision_summary, content_schema_version, content_json, created_at
FROM advice_draft_section_versions
WHERE draft_id = ?
  AND user_id = ?
ORDER BY draft_revision_no ASC, FIELD(section_type, 'outfit', 'hair', 'makeup'), id ASC
`, draft.ID, userID); err != nil {
		return nil, err
	}
	revisions := make([]DraftRevision, 0)
	revisionIndex := map[int]int{}
	for _, row := range rows {
		section, err := row.section()
		if err != nil {
			return nil, err
		}
		index, ok := revisionIndex[row.DraftRevisionNo]
		if !ok {
			revisions = append(revisions, DraftRevision{
				DraftRevisionNo: row.DraftRevisionNo,
				SourceMsgID:     row.SourceMsgID.Int64,
				UserIntent:      row.UserIntent.String,
				RevisionSummary: row.RevisionSummary.String,
				CreatedAt:       row.CreatedAt.Time,
			})
			index = len(revisions) - 1
			revisionIndex[row.DraftRevisionNo] = index
		}
		if revisions[index].CreatedAt.IsZero() && row.CreatedAt.Valid {
			revisions[index].CreatedAt = row.CreatedAt.Time
		}
		revisions[index].Sections = append(revisions[index].Sections, section)
	}
	return revisions, nil
}

func (r *MySQLRepository) ConfirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error) {
	if r == nil || r.ext == nil {
		return Advice{}, ErrRepositoryUnsupported
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Advice{}, err
		}
		advice, err := (&MySQLRepository{ext: tx}).confirmDraft(ctx, userID, publicID)
		if err != nil {
			_ = tx.Rollback()
			return Advice{}, err
		}
		if err := tx.Commit(); err != nil {
			return Advice{}, err
		}
		return advice, nil
	}
	return r.confirmDraft(ctx, userID, publicID)
}

func (r *MySQLRepository) DiscardDraft(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return ErrRepositoryUnsupported
	}
	return requireAffected(r.ext.ExecContext(ctx, `
UPDATE advice_drafts
SET status = ?, updated_at = ?
WHERE user_id = ?
  AND public_id = ?
  AND status = ?
  AND deleted_at IS NULL
`, DraftStatusDiscarded, time.Now().UTC(), userID, publicID, DraftStatusDraft))
}

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

func (r *MySQLRepository) createDraft(ctx context.Context, input CreateDraftInput) (Draft, error) {
	publicID := id.NewPublicID("drf")
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_drafts
  (public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no)
VALUES
  (?, ?, NULLIF(?, 0), ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
`, publicID, input.UserID, input.SourceMsgID, DraftStatusDraft, input.SceneKey, input.SceneLabel, input.TargetDate, input.Occasion, input.WeatherText, input.MoodText, input.StyleGoal, input.AvoidGoal, 1)
	if err != nil {
		return Draft{}, err
	}
	draftID, err := dbutil.RequireLastInsertID(result, "advice draft create")
	if err != nil {
		return Draft{}, err
	}
	draft := Draft{
		ID:                draftID,
		PublicID:          publicID,
		UserID:            input.UserID,
		SourceMsgID:       input.SourceMsgID,
		Status:            DraftStatusDraft,
		SceneKey:          input.SceneKey,
		SceneLabel:        input.SceneLabel,
		TargetDate:        input.TargetDate,
		Occasion:          input.Occasion,
		WeatherText:       input.WeatherText,
		MoodText:          input.MoodText,
		StyleGoal:         input.StyleGoal,
		AvoidGoal:         input.AvoidGoal,
		CurrentRevisionNo: 1,
	}
	for _, sectionInput := range input.Sections {
		section, err := r.createInitialSection(ctx, draft, sectionInput, input)
		if err != nil {
			return Draft{}, err
		}
		draft.Sections = append(draft.Sections, section)
	}
	return draft, nil
}

func (r *MySQLRepository) createInitialSection(ctx context.Context, draft Draft, sectionInput DraftSectionInput, input CreateDraftInput) (DraftSection, error) {
	content, err := jsonText(sectionInput.ContentJSON)
	if err != nil {
		return DraftSection{}, err
	}
	sectionPublicID := id.NewPublicID("ads")
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_draft_sections
  (public_id, draft_id, user_id, section_type, current_section_version_id, current_section_version_no, content_schema_version, content_json)
VALUES
  (?, ?, ?, ?, NULL, ?, ?, CAST(? AS JSON))
`, sectionPublicID, draft.ID, draft.UserID, sectionInput.SectionType, 1, schemaVersionOrDefault(sectionInput.ContentSchemaVersion), content)
	if err != nil {
		return DraftSection{}, err
	}
	sectionID, err := dbutil.RequireLastInsertID(result, "advice draft section create")
	if err != nil {
		return DraftSection{}, err
	}
	versionPublicID := id.NewPublicID("adsv")
	result, err = r.ext.ExecContext(ctx, `
INSERT INTO advice_draft_section_versions
  (public_id, draft_id, section_id, user_id, section_type, section_version_no, draft_revision_no, source_msg_id, user_intent, revision_summary, content_schema_version, content_json)
VALUES
  (?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, ''), NULLIF(?, ''), ?, CAST(? AS JSON))
`, versionPublicID, draft.ID, sectionID, draft.UserID, sectionInput.SectionType, 1, 1, input.SourceMsgID, input.UserIntent, firstNonEmpty(sectionInput.RevisionSummary, input.RevisionSummary), schemaVersionOrDefault(sectionInput.ContentSchemaVersion), content)
	if err != nil {
		return DraftSection{}, err
	}
	versionID, err := dbutil.RequireLastInsertID(result, "advice draft section version create")
	if err != nil {
		return DraftSection{}, err
	}
	if err := requireAffected(r.ext.ExecContext(ctx, `
UPDATE advice_draft_sections
SET current_section_version_id = ?
WHERE id = ?
`, versionID, sectionID)); err != nil {
		return DraftSection{}, err
	}
	return DraftSection{
		ID:                      sectionID,
		PublicID:                sectionPublicID,
		DraftID:                 draft.ID,
		UserID:                  draft.UserID,
		SectionType:             sectionInput.SectionType,
		CurrentSectionVersionID: versionID,
		CurrentSectionVersionNo: 1,
		ContentSchemaVersion:    schemaVersionOrDefault(sectionInput.ContentSchemaVersion),
		ContentJSON:             sectionInput.ContentJSON,
	}, nil
}

func (r *MySQLRepository) updateDraftSections(ctx context.Context, input UpdateDraftInput) (Draft, error) {
	current, err := r.findDraftForUser(ctx, input.UserID, input.PublicID)
	if err != nil {
		return Draft{}, err
	}
	if current.Status != DraftStatusDraft {
		return Draft{}, ErrDraftNotActive
	}
	sectionStates, err := r.listSectionStates(ctx, current.ID)
	if err != nil {
		return Draft{}, err
	}
	nextRevision := current.CurrentRevisionNo + 1
	for _, sectionInput := range input.Sections {
		state, ok := sectionStates[sectionInput.SectionType]
		if !ok {
			return Draft{}, ErrDraftNotFound
		}
		content, err := jsonText(sectionInput.ContentJSON)
		if err != nil {
			return Draft{}, err
		}
		nextSectionVersion := state.CurrentSectionVersionNo + 1
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_draft_section_versions
  (public_id, draft_id, section_id, user_id, section_type, section_version_no, draft_revision_no, source_msg_id, user_intent, revision_summary, content_schema_version, content_json)
VALUES
  (?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, ''), NULLIF(?, ''), ?, CAST(? AS JSON))
`, id.NewPublicID("adsv"), current.ID, state.ID, input.UserID, sectionInput.SectionType, nextSectionVersion, nextRevision, input.SourceMsgID, input.UserIntent, firstNonEmpty(sectionInput.RevisionSummary, input.RevisionSummary), schemaVersionOrDefault(sectionInput.ContentSchemaVersion), content)
		if err != nil {
			return Draft{}, err
		}
		versionID, err := dbutil.RequireLastInsertID(result, "advice draft section version create")
		if err != nil {
			return Draft{}, err
		}
		if err := requireAffected(r.ext.ExecContext(ctx, `
UPDATE advice_draft_sections
SET current_section_version_id = ?,
    current_section_version_no = ?,
    content_schema_version = ?,
    content_json = CAST(? AS JSON),
    updated_at = ?
WHERE id = ?
`, versionID, nextSectionVersion, schemaVersionOrDefault(sectionInput.ContentSchemaVersion), content, time.Now().UTC(), state.ID)); err != nil {
			return Draft{}, err
		}
	}
	if err := requireAffected(r.ext.ExecContext(ctx, `
UPDATE advice_drafts
SET current_revision_no = ?,
    updated_at = ?
WHERE id = ?
`, nextRevision, time.Now().UTC(), current.ID)); err != nil {
		return Draft{}, err
	}
	return r.findDraftWithSections(ctx, input.UserID, input.PublicID)
}

func (r *MySQLRepository) confirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error) {
	draft, err := r.findDraftForUser(ctx, userID, publicID)
	if err != nil {
		return Advice{}, err
	}
	if draft.Status == DraftStatusConfirmed && draft.ConfirmedAdviceID != 0 {
		return r.findAdviceByID(ctx, userID, draft.ConfirmedAdviceID)
	}
	if draft.Status != DraftStatusDraft {
		return Advice{}, ErrDraftNotActive
	}
	sections, err := r.listDraftSections(ctx, draft.ID)
	if err != nil {
		return Advice{}, err
	}
	if !hasRequiredSections(sections) {
		return Advice{}, ErrDraftIncomplete
	}
	advicePublicID := id.NewPublicID("adv")
	result, err := r.ext.ExecContext(ctx, `
INSERT INTO advices
  (public_id, user_id, source_msg_id, source_draft_id, source_draft_revision_no, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal)
VALUES
  (?, ?, NULLIF(?, 0), ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''))
`, advicePublicID, userID, draft.SourceMsgID, draft.ID, draft.CurrentRevisionNo, AdviceStatusReady, draft.SceneKey, draft.SceneLabel, draft.TargetDate, draft.Occasion, draft.WeatherText, draft.MoodText, draft.StyleGoal, draft.AvoidGoal)
	if err != nil {
		return Advice{}, err
	}
	adviceID, err := dbutil.RequireLastInsertID(result, "advice create")
	if err != nil {
		return Advice{}, err
	}
	advice := Advice{
		ID:                    adviceID,
		PublicID:              advicePublicID,
		UserID:                userID,
		SourceMsgID:           draft.SourceMsgID,
		SourceDraftID:         draft.ID,
		SourceDraftRevisionNo: draft.CurrentRevisionNo,
		Status:                AdviceStatusReady,
		SceneKey:              draft.SceneKey,
		SceneLabel:            draft.SceneLabel,
		TargetDate:            draft.TargetDate,
		Occasion:              draft.Occasion,
		WeatherText:           draft.WeatherText,
		MoodText:              draft.MoodText,
		StyleGoal:             draft.StyleGoal,
		AvoidGoal:             draft.AvoidGoal,
	}
	for _, section := range sections {
		content, err := jsonText(section.ContentJSON)
		if err != nil {
			return Advice{}, err
		}
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_sections
  (public_id, advice_id, user_id, section_type, source_draft_section_version_id, content_schema_version, content_json)
VALUES
  (?, ?, ?, ?, ?, ?, CAST(? AS JSON))
`, id.NewPublicID("adsn"), adviceID, userID, section.SectionType, section.CurrentSectionVersionID, section.ContentSchemaVersion, content)
		if err != nil {
			return Advice{}, err
		}
		adviceSectionID, err := dbutil.RequireLastInsertID(result, "advice section create")
		if err != nil {
			return Advice{}, err
		}
		if section.SectionType == SectionTypeOutfit {
			if err := r.createConfirmedOutfitRefs(ctx, userID, adviceID, adviceSectionID, section); err != nil {
				return Advice{}, err
			}
		}
		advice.Sections = append(advice.Sections, AdviceSection{
			ID:                          adviceSectionID,
			AdviceID:                    adviceID,
			UserID:                      userID,
			SectionType:                 section.SectionType,
			SourceDraftSectionVersionID: section.CurrentSectionVersionID,
			ContentSchemaVersion:        section.ContentSchemaVersion,
			ContentJSON:                 section.ContentJSON,
		})
	}
	if err := requireAffected(r.ext.ExecContext(ctx, `
UPDATE advice_drafts
SET status = ?,
    confirmed_advice_id = ?,
    updated_at = ?
WHERE id = ?
  AND status = ?
`, DraftStatusConfirmed, adviceID, time.Now().UTC(), draft.ID, DraftStatusDraft)); err != nil {
		return Advice{}, err
	}
	return advice, nil
}

func (r *MySQLRepository) createConfirmedOutfitRefs(ctx context.Context, userID, adviceID, adviceSectionID int64, section DraftSection) error {
	items, ok := outfitItems(section.ContentJSON)
	if !ok {
		return nil
	}
	for index, item := range items {
		ref := outfitItemRef(item)
		switch ref.SourceType {
		case "wardrobe_item":
			if ref.SourcePublicID == "" {
				continue
			}
			clothesID, err := r.findActiveClothesID(ctx, userID, ref.SourcePublicID)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if _, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_clothes_refs
  (public_id, advice_id, advice_section_id, user_id, clothes_id, role, display_text, reason_text, sort_order, source_draft_section_version_id)
VALUES
  (?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)
`, id.NewPublicID("acr"), adviceID, adviceSectionID, userID, clothesID, ref.Role, ref.Text, ref.ReasonText, index+1, section.CurrentSectionVersionID); err != nil {
				return err
			}
		case "gap_item":
			if ref.Text == "" {
				continue
			}
			if _, err := r.ext.ExecContext(ctx, `
INSERT INTO advice_gap_refs
  (public_id, advice_id, advice_section_id, user_id, role, display_text, reason_text, sort_order, source_draft_section_version_id)
VALUES
  (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)
`, id.NewPublicID("agr"), adviceID, adviceSectionID, userID, ref.Role, ref.Text, ref.ReasonText, index+1, section.CurrentSectionVersionID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *MySQLRepository) findActiveClothesID(ctx context.Context, userID int64, publicID string) (int64, error) {
	var clothesID int64
	err := sqlx.GetContext(ctx, r.ext, &clothesID, `
SELECT id FROM clothes
WHERE user_id = ?
  AND public_id = ?
  AND status = 'active'
  AND recommendation_status <> 'paused'
  AND deleted_at IS NULL
LIMIT 1
`, userID, publicID)
	return clothesID, err
}

func (r *MySQLRepository) findCurrentDraft(ctx context.Context, userID int64) (Draft, error) {
	var row draftRow
	err := sqlx.GetContext(ctx, r.ext, &row, draftSelectSQL()+`
WHERE user_id = ?
  AND status = ?
  AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT 1
`, userID, DraftStatusDraft)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrDraftNotFound
	}
	if err != nil {
		return Draft{}, err
	}
	return row.draft(), nil
}

func (r *MySQLRepository) findDraftWithSections(ctx context.Context, userID int64, publicID string) (Draft, error) {
	draft, err := r.findDraftForUser(ctx, userID, publicID)
	if err != nil {
		return Draft{}, err
	}
	sections, err := r.listDraftSections(ctx, draft.ID)
	if err != nil {
		return Draft{}, err
	}
	draft.Sections = sections
	return draft, nil
}

func (r *MySQLRepository) findDraftForUser(ctx context.Context, userID int64, publicID string) (Draft, error) {
	var row draftRow
	err := sqlx.GetContext(ctx, r.ext, &row, draftSelectSQL()+`
WHERE user_id = ?
  AND public_id = ?
  AND deleted_at IS NULL
LIMIT 1
`, userID, publicID)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, ErrDraftNotFound
	}
	if err != nil {
		return Draft{}, err
	}
	return row.draft(), nil
}

func (r *MySQLRepository) findAdviceByID(ctx context.Context, userID int64, adviceID int64) (Advice, error) {
	var row adviceRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT id, public_id, user_id, source_msg_id, source_draft_id, source_draft_revision_no, status, scene_key, scene_label, occasion, weather_text, mood_text, style_goal, avoid_goal, created_at, updated_at
FROM advices
WHERE user_id = ?
  AND id = ?
  AND deleted_at IS NULL
LIMIT 1
`, userID, adviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return Advice{}, ErrDraftNotFound
	}
	if err != nil {
		return Advice{}, err
	}
	return row.advice(), nil
}

func (r *MySQLRepository) listDraftSections(ctx context.Context, draftID int64) ([]DraftSection, error) {
	var rows []draftSectionRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, sectionSelectSQL()+`
WHERE draft_id = ?
  AND deleted_at IS NULL
ORDER BY FIELD(section_type, 'outfit', 'hair', 'makeup'), id ASC
`, draftID); err != nil {
		return nil, err
	}
	sections := make([]DraftSection, 0, len(rows))
	for _, row := range rows {
		section, err := row.section()
		if err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func (r *MySQLRepository) listSectionStates(ctx context.Context, draftID int64) (map[string]sectionState, error) {
	var rows []sectionState
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT id, section_type, current_section_version_no
FROM advice_draft_sections
WHERE draft_id = ?
  AND deleted_at IS NULL
`, draftID); err != nil {
		return nil, err
	}
	result := map[string]sectionState{}
	for _, row := range rows {
		result[row.SectionType] = row
	}
	return result, nil
}

func draftSelectSQL() string {
	return `
SELECT id, public_id, user_id, source_msg_id, status, scene_key, scene_label, target_date, occasion, weather_text, mood_text, style_goal, avoid_goal, current_revision_no, confirmed_advice_id, created_at, updated_at
FROM advice_drafts
`
}

func sectionSelectSQL() string {
	return `
SELECT id, public_id, draft_id, user_id, section_type, current_section_version_id, current_section_version_no, content_schema_version, content_json, created_at, updated_at
FROM advice_draft_sections
`
}

type draftRow struct {
	ID                int64          `db:"id"`
	PublicID          string         `db:"public_id"`
	UserID            int64          `db:"user_id"`
	SourceMsgID       sql.NullInt64  `db:"source_msg_id"`
	Status            string         `db:"status"`
	SceneKey          sql.NullString `db:"scene_key"`
	SceneLabel        sql.NullString `db:"scene_label"`
	TargetDate        sql.NullTime   `db:"target_date"`
	Occasion          sql.NullString `db:"occasion"`
	WeatherText       sql.NullString `db:"weather_text"`
	MoodText          sql.NullString `db:"mood_text"`
	StyleGoal         sql.NullString `db:"style_goal"`
	AvoidGoal         sql.NullString `db:"avoid_goal"`
	CurrentRevisionNo int            `db:"current_revision_no"`
	ConfirmedAdviceID sql.NullInt64  `db:"confirmed_advice_id"`
	CreatedAt         sql.NullTime   `db:"created_at"`
	UpdatedAt         sql.NullTime   `db:"updated_at"`
}

type chatMessageRow struct {
	ID              int64          `db:"id"`
	PublicID        string         `db:"public_id"`
	UserID          int64          `db:"user_id"`
	SourceMsgID     sql.NullInt64  `db:"source_msg_id"`
	Role            string         `db:"role"`
	MsgType         string         `db:"msg_type"`
	ContentText     sql.NullString `db:"content_text"`
	RelatedType     sql.NullString `db:"related_type"`
	RelatedID       sql.NullInt64  `db:"related_id"`
	RelatedPublicID sql.NullString `db:"related_public_id"`
	Status          string         `db:"status"`
}

func (r chatMessageRow) message() ChatMessage {
	return ChatMessage{
		ID:              r.ID,
		PublicID:        r.PublicID,
		UserID:          r.UserID,
		SourceMsgID:     r.SourceMsgID.Int64,
		Role:            r.Role,
		MsgType:         r.MsgType,
		ContentText:     r.ContentText.String,
		RelatedType:     r.RelatedType.String,
		RelatedID:       r.RelatedID.Int64,
		RelatedPublicID: r.RelatedPublicID.String,
		Status:          r.Status,
	}
}

func (r draftRow) draft() Draft {
	draft := Draft{
		ID:                r.ID,
		PublicID:          r.PublicID,
		UserID:            r.UserID,
		SourceMsgID:       r.SourceMsgID.Int64,
		Status:            r.Status,
		SceneKey:          r.SceneKey.String,
		SceneLabel:        r.SceneLabel.String,
		TargetDate:        nil,
		Occasion:          r.Occasion.String,
		WeatherText:       r.WeatherText.String,
		MoodText:          r.MoodText.String,
		StyleGoal:         r.StyleGoal.String,
		AvoidGoal:         r.AvoidGoal.String,
		CurrentRevisionNo: r.CurrentRevisionNo,
		ConfirmedAdviceID: r.ConfirmedAdviceID.Int64,
	}
	if r.CreatedAt.Valid {
		draft.CreatedAt = r.CreatedAt.Time
	}
	if r.TargetDate.Valid {
		targetDate := r.TargetDate.Time
		draft.TargetDate = &targetDate
	}
	if r.UpdatedAt.Valid {
		draft.UpdatedAt = r.UpdatedAt.Time
	}
	return draft
}

type draftSectionRow struct {
	ID                      int64          `db:"id"`
	PublicID                string         `db:"public_id"`
	DraftID                 int64          `db:"draft_id"`
	UserID                  int64          `db:"user_id"`
	SectionType             string         `db:"section_type"`
	CurrentSectionVersionID sql.NullInt64  `db:"current_section_version_id"`
	CurrentSectionVersionNo int            `db:"current_section_version_no"`
	ContentSchemaVersion    string         `db:"content_schema_version"`
	ContentJSON             sql.NullString `db:"content_json"`
	CreatedAt               sql.NullTime   `db:"created_at"`
	UpdatedAt               sql.NullTime   `db:"updated_at"`
}

type draftSectionVersionRow struct {
	PublicID             string         `db:"public_id"`
	SectionType          string         `db:"section_type"`
	SectionVersionNo     int            `db:"section_version_no"`
	DraftRevisionNo      int            `db:"draft_revision_no"`
	SourceMsgID          sql.NullInt64  `db:"source_msg_id"`
	UserIntent           sql.NullString `db:"user_intent"`
	RevisionSummary      sql.NullString `db:"revision_summary"`
	ContentSchemaVersion string         `db:"content_schema_version"`
	ContentJSON          sql.NullString `db:"content_json"`
	CreatedAt            sql.NullTime   `db:"created_at"`
}

type adviceRow struct {
	ID                    int64          `db:"id"`
	PublicID              string         `db:"public_id"`
	UserID                int64          `db:"user_id"`
	SourceMsgID           sql.NullInt64  `db:"source_msg_id"`
	SourceDraftID         sql.NullInt64  `db:"source_draft_id"`
	SourceDraftRevisionNo sql.NullInt64  `db:"source_draft_revision_no"`
	Status                string         `db:"status"`
	SceneKey              sql.NullString `db:"scene_key"`
	SceneLabel            sql.NullString `db:"scene_label"`
	Occasion              sql.NullString `db:"occasion"`
	WeatherText           sql.NullString `db:"weather_text"`
	MoodText              sql.NullString `db:"mood_text"`
	StyleGoal             sql.NullString `db:"style_goal"`
	AvoidGoal             sql.NullString `db:"avoid_goal"`
	CreatedAt             sql.NullTime   `db:"created_at"`
	UpdatedAt             sql.NullTime   `db:"updated_at"`
}

func (r adviceRow) advice() Advice {
	advice := Advice{
		ID:                    r.ID,
		PublicID:              r.PublicID,
		UserID:                r.UserID,
		SourceMsgID:           r.SourceMsgID.Int64,
		SourceDraftID:         r.SourceDraftID.Int64,
		SourceDraftRevisionNo: int(r.SourceDraftRevisionNo.Int64),
		Status:                r.Status,
		SceneKey:              r.SceneKey.String,
		SceneLabel:            r.SceneLabel.String,
		Occasion:              r.Occasion.String,
		WeatherText:           r.WeatherText.String,
		MoodText:              r.MoodText.String,
		StyleGoal:             r.StyleGoal.String,
		AvoidGoal:             r.AvoidGoal.String,
	}
	if r.CreatedAt.Valid {
		advice.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		advice.UpdatedAt = r.UpdatedAt.Time
	}
	return advice
}

func (r draftSectionRow) section() (DraftSection, error) {
	content := map[string]any{}
	if r.ContentJSON.Valid && r.ContentJSON.String != "" {
		if err := json.Unmarshal([]byte(r.ContentJSON.String), &content); err != nil {
			return DraftSection{}, err
		}
	}
	section := DraftSection{
		ID:                      r.ID,
		PublicID:                r.PublicID,
		DraftID:                 r.DraftID,
		UserID:                  r.UserID,
		SectionType:             r.SectionType,
		CurrentSectionVersionID: r.CurrentSectionVersionID.Int64,
		CurrentSectionVersionNo: r.CurrentSectionVersionNo,
		ContentSchemaVersion:    r.ContentSchemaVersion,
		ContentJSON:             content,
	}
	if r.CreatedAt.Valid {
		section.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		section.UpdatedAt = r.UpdatedAt.Time
	}
	return section, nil
}

func (r draftSectionVersionRow) section() (DraftVersionSection, error) {
	content := map[string]any{}
	if r.ContentJSON.Valid && r.ContentJSON.String != "" {
		if err := json.Unmarshal([]byte(r.ContentJSON.String), &content); err != nil {
			return DraftVersionSection{}, err
		}
	}
	section := DraftVersionSection{
		PublicID:             r.PublicID,
		SectionType:          r.SectionType,
		SectionVersionNo:     r.SectionVersionNo,
		ContentSchemaVersion: r.ContentSchemaVersion,
		ContentJSON:          content,
	}
	if r.CreatedAt.Valid {
		section.CreatedAt = r.CreatedAt.Time
	}
	return section, nil
}

type sectionState struct {
	ID                      int64  `db:"id"`
	SectionType             string `db:"section_type"`
	CurrentSectionVersionNo int    `db:"current_section_version_no"`
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDraftNotFound
	}
	return nil
}

func jsonText(value any) (string, error) {
	if value == nil {
		return "null", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func schemaVersionOrDefault(version string) string {
	if version == "" {
		return "v1"
	}
	return version
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func hasRequiredSections(sections []DraftSection) bool {
	seen := map[string]bool{}
	for _, section := range sections {
		seen[section.SectionType] = true
	}
	return seen[SectionTypeOutfit] && seen[SectionTypeHair] && seen[SectionTypeMakeup]
}

func validateCreateDraftInput(input CreateDraftInput) error {
	if len(input.Sections) == 0 {
		return ErrDraftInvalid
	}
	seen := map[string]bool{}
	for _, section := range input.Sections {
		if err := validateDraftSectionInput(section); err != nil {
			return err
		}
		if seen[section.SectionType] {
			return ErrDraftInvalid
		}
		seen[section.SectionType] = true
	}
	if !seen[SectionTypeOutfit] || !seen[SectionTypeHair] || !seen[SectionTypeMakeup] {
		return ErrDraftInvalid
	}
	return nil
}

func validateUpdateDraftInput(input UpdateDraftInput) error {
	if len(input.Sections) == 0 {
		return ErrDraftInvalid
	}
	seen := map[string]bool{}
	for _, section := range input.Sections {
		if err := validateDraftSectionInput(section); err != nil {
			return err
		}
		if seen[section.SectionType] {
			return ErrDraftInvalid
		}
		seen[section.SectionType] = true
	}
	return nil
}

func validateDraftSectionInput(section DraftSectionInput) error {
	switch section.SectionType {
	case SectionTypeOutfit, SectionTypeHair, SectionTypeMakeup:
	default:
		return ErrDraftInvalid
	}
	content := section.ContentJSON
	if content == nil {
		return ErrDraftInvalid
	}
	allowed := map[string]bool{
		"title":            true,
		"summary":          true,
		"why_text":         true,
		"avoid_text":       true,
		"alternative_text": true,
		"items":            true,
	}
	for key := range content {
		if !allowed[key] {
			return ErrDraftInvalid
		}
	}
	for _, key := range []string{"title", "summary", "why_text", "avoid_text", "alternative_text"} {
		if jsonString(content[key]) == "" {
			return ErrDraftInvalid
		}
	}
	if rawItems, ok := content["items"]; ok {
		items, ok := rawItems.([]any)
		if !ok {
			return ErrDraftInvalid
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				return ErrDraftInvalid
			}
			for key := range item {
				switch key {
				case "role", "text", "source_type", "source_public_id", "reason_text":
				default:
					return ErrDraftInvalid
				}
			}
		}
	}
	return nil
}

type confirmedOutfitRef struct {
	Role           string
	Text           string
	SourceType     string
	SourcePublicID string
	ReasonText     string
}

func outfitItems(content map[string]any) ([]map[string]any, bool) {
	rawItems, ok := content["items"].([]any)
	if !ok || len(rawItems) == 0 {
		return nil, false
	}
	items := make([]map[string]any, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	return items, len(items) > 0
}

func outfitItemRef(item map[string]any) confirmedOutfitRef {
	return confirmedOutfitRef{
		Role:           jsonString(item["role"]),
		Text:           jsonString(item["text"]),
		SourceType:     jsonString(item["source_type"]),
		SourcePublicID: jsonString(item["source_public_id"]),
		ReasonText:     jsonString(item["reason_text"]),
	}
}

func jsonString(value any) string {
	if value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
