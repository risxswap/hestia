package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"hestia/server/internal/common/dbutil"

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

func (r *MySQLRepository) Summary(ctx context.Context, userID int64) (Summary, error) {
	if r == nil || r.ext == nil {
		return Summary{}, errors.New("profile repository database is nil")
	}
	var row profileSummaryRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT
  u.public_id AS user_public_id,
  COALESCE(u.nickname, '') AS nickname,
  u.onboarding_status,
  p.id AS profile_id,
  p.public_id AS profile_public_id,
  p.gender,
  p.height_cm,
  p.body_notes,
  p.skin_notes,
  p.hair_notes,
  p.lifestyle_scenarios,
  p.style_goal_summary
FROM users u
LEFT JOIN profiles p
  ON p.user_id = u.id
  AND p.deleted_at IS NULL
WHERE u.id = ?
  AND u.deleted_at IS NULL
LIMIT 1
`, userID)
	if err != nil {
		return Summary{}, err
	}
	summary, err := row.toSummary()
	if err != nil {
		return Summary{}, err
	}
	memory, err := r.memorySummary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.MemorySummary = memory
	latest, err := r.latestReportSummary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.LatestReport = latest
	return summary, nil
}

func (r *MySQLRepository) Upsert(ctx context.Context, item Profile) (Profile, error) {
	if r == nil || r.ext == nil {
		return Profile{}, errors.New("profile repository database is nil")
	}
	scenarios, err := jsonText(item.LifestyleScenarios)
	if err != nil {
		return Profile{}, err
	}
	_, err = r.ext.ExecContext(ctx, `
INSERT INTO profiles
  (public_id, user_id, status, gender, height_cm, body_notes, skin_notes, hair_notes, lifestyle_scenarios, style_goal_summary)
VALUES
  (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), NULLIF(?, ''))
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  gender = VALUES(gender),
  height_cm = VALUES(height_cm),
  body_notes = VALUES(body_notes),
  skin_notes = VALUES(skin_notes),
  hair_notes = VALUES(hair_notes),
  lifestyle_scenarios = VALUES(lifestyle_scenarios),
  style_goal_summary = VALUES(style_goal_summary)
`, item.PublicID, item.UserID, item.Status, item.Gender, item.HeightCM, item.BodyNotes, item.SkinNotes, item.HairNotes, scenarios, item.StyleGoalSummary)
	if err != nil {
		return Profile{}, err
	}
	var saved Profile
	err = sqlx.GetContext(ctx, r.ext, &saved, `
SELECT
  id,
  public_id,
  user_id,
  status,
  COALESCE(gender, '') AS gender,
  height_cm,
  COALESCE(body_notes, '') AS body_notes,
  COALESCE(skin_notes, '') AS skin_notes,
  COALESCE(hair_notes, '') AS hair_notes,
  COALESCE(style_goal_summary, '') AS style_goal_summary
FROM profiles
WHERE user_id = ? AND deleted_at IS NULL
LIMIT 1
`, item.UserID)
	if err != nil {
		return Profile{}, err
	}
	saved.LifestyleScenarios = item.LifestyleScenarios
	return saved, nil
}

func (r *MySQLRepository) ReplaceFacts(ctx context.Context, userID int64, profileID int64, facts []Fact) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	if _, err := r.ext.ExecContext(ctx, `UPDATE profile_facts SET deleted_at = CURRENT_TIMESTAMP(3) WHERE user_id = ? AND profile_id = ? AND deleted_at IS NULL`, userID, profileID); err != nil {
		return err
	}
	for _, fact := range facts {
		raw, err := jsonText(fact.Value)
		if err != nil {
			return err
		}
		if _, err := r.ext.ExecContext(ctx, `
INSERT INTO profile_facts (user_id, profile_id, fact_key, fact_value, source, confirmed_at)
VALUES (?, ?, ?, CAST(? AS JSON), ?, CURRENT_TIMESTAMP(3))
`, userID, profileID, fact.Key, raw, fact.Source); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) ReplacePrefs(ctx context.Context, userID int64, profileID int64, prefs []Pref) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	if _, err := r.ext.ExecContext(ctx, `UPDATE profile_prefs SET deleted_at = CURRENT_TIMESTAMP(3) WHERE user_id = ? AND profile_id = ? AND deleted_at IS NULL`, userID, profileID); err != nil {
		return err
	}
	for _, pref := range prefs {
		raw, err := jsonText(pref.Value)
		if err != nil {
			return err
		}
		if _, err := r.ext.ExecContext(ctx, `
INSERT INTO profile_prefs (user_id, profile_id, pref_type, pref_key, pref_value, polarity, source, confidence)
VALUES (?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?)
`, userID, profileID, pref.Type, pref.Key, raw, pref.Polarity, pref.Source, pref.Confidence); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) CreateInferences(ctx context.Context, inferences []Inference) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	for _, inference := range inferences {
		raw, err := jsonText(inference.Value)
		if err != nil {
			return err
		}
		if _, err := r.ext.ExecContext(ctx, `
INSERT INTO profile_inferences
  (user_id, profile_id, inference_key, inference_value, confidence, source_job_id, status)
VALUES
  (?, ?, ?, CAST(? AS JSON), ?, ?, 'active')
`, inference.UserID, inference.ProfileID, inference.Key, raw, inference.Confidence, inference.SourceJobID); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) MarkUserOnboardingCompleted(ctx context.Context, userID int64) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `UPDATE users SET onboarding_status = 'completed' WHERE id = ? AND deleted_at IS NULL`, userID)
	if err != nil {
		return err
	}
	return dbutil.RequireRowsAffected(result, "user onboarding complete")
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

func (r *MySQLRepository) memorySummary(ctx context.Context, userID int64) (MemorySummary, error) {
	var summary MemorySummary
	err := sqlx.GetContext(ctx, r.ext, &summary, `
SELECT
  (SELECT COUNT(*) FROM profile_facts WHERE user_id = ? AND deleted_at IS NULL) AS fact_count,
  (SELECT COUNT(*) FROM profile_prefs WHERE user_id = ? AND polarity = 'positive' AND deleted_at IS NULL) AS preference_count,
  (SELECT COUNT(*) FROM profile_prefs WHERE user_id = ? AND polarity = 'negative' AND deleted_at IS NULL) AS avoidance_count,
  (SELECT COUNT(*) FROM profile_inferences WHERE user_id = ? AND status = 'active' AND deleted_at IS NULL) AS inference_count,
  (SELECT COUNT(*) FROM profile_inferences WHERE user_id = ? AND status = 'active' AND confirmed_by_user = 0 AND deleted_at IS NULL) AS pending_confirmation_count
`, userID, userID, userID, userID, userID)
	return summary, err
}

func (r *MySQLRepository) latestReportSummary(ctx context.Context, userID int64) (*LatestReportSummary, error) {
	var row latestReportSummaryRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT
  public_id,
  title,
  status,
  generated_at
FROM reports
WHERE user_id = ?
  AND report_type = 'initial'
  AND status = 'ready'
  AND deleted_at IS NULL
ORDER BY generated_at DESC, id DESC
LIMIT 1
`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	report := &LatestReportSummary{
		PublicID: row.PublicID,
		Title:    row.Title,
		Status:   row.Status,
	}
	if row.GeneratedAt.Valid {
		report.GeneratedAt = &row.GeneratedAt.Time
	}
	return report, nil
}

type profileSummaryRow struct {
	UserPublicID       string          `db:"user_public_id"`
	Nickname           string          `db:"nickname"`
	OnboardingStatus   string          `db:"onboarding_status"`
	ProfileID          sql.NullInt64   `db:"profile_id"`
	ProfilePublicID    sql.NullString  `db:"profile_public_id"`
	Gender             sql.NullString  `db:"gender"`
	HeightCM           sql.NullInt64   `db:"height_cm"`
	BodyNotes          sql.NullString  `db:"body_notes"`
	SkinNotes          sql.NullString  `db:"skin_notes"`
	HairNotes          sql.NullString  `db:"hair_notes"`
	LifestyleScenarios json.RawMessage `db:"lifestyle_scenarios"`
	StyleGoalSummary   sql.NullString  `db:"style_goal_summary"`
}

func (r profileSummaryRow) toSummary() (Summary, error) {
	summary := Summary{
		User: UserSummary{
			UserPublicID:     r.UserPublicID,
			Nickname:         r.Nickname,
			OnboardingStatus: r.OnboardingStatus,
		},
	}
	if !r.ProfileID.Valid {
		return summary, nil
	}
	scenarios, err := decodeStringSlice(r.LifestyleScenarios)
	if err != nil {
		return Summary{}, err
	}
	profile := &ProfileSummary{
		ProfilePublicID:    r.ProfilePublicID.String,
		Gender:             r.Gender.String,
		BodyNotes:          r.BodyNotes.String,
		SkinNotes:          r.SkinNotes.String,
		HairNotes:          r.HairNotes.String,
		LifestyleScenarios: scenarios,
		StyleGoalSummary:   r.StyleGoalSummary.String,
	}
	if r.HeightCM.Valid {
		height := int(r.HeightCM.Int64)
		profile.HeightCM = &height
	}
	summary.Profile = profile
	return summary, nil
}

type latestReportSummaryRow struct {
	PublicID    string       `db:"public_id"`
	Title       string       `db:"title"`
	Status      string       `db:"status"`
	GeneratedAt sql.NullTime `db:"generated_at"`
}

func decodeStringSlice(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}
