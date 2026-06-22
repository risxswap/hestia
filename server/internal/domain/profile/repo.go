package profile

import (
	"context"
	"encoding/json"
	"errors"

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
	_, err := r.ext.ExecContext(ctx, `UPDATE users SET onboarding_status = 'completed' WHERE id = ? AND deleted_at IS NULL`, userID)
	return err
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
