package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

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
  p.weight_kg,
  p.body_notes,
  p.skin_notes,
  p.hair_notes,
  p.face_shape,
  p.upper_body_notes,
  p.lower_body_notes,
  p.size_notes,
  p.lifestyle_scenarios,
  p.style_goal_summary
FROM users u
LEFT JOIN profiles p
  ON p.user_id = u.id
  AND p.status = 'active'
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
	preferences, err := r.preferencesSummary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.Preferences = preferences
	latest, err := r.latestReportSummary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.LatestReport = latest
	photos, err := r.profilePhotos(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.ProfilePhotos = photos
	return summary, nil
}

func (r *MySQLRepository) UpdateExplicitProfile(ctx context.Context, userID int64, input UpdateProfileInput) (Summary, error) {
	if r == nil || r.ext == nil {
		return Summary{}, errors.New("profile repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Summary{}, err
		}
		txRepo := &MySQLRepository{ext: tx}
		if err := txRepo.updateExplicitProfile(ctx, userID, input); err != nil {
			_ = tx.Rollback()
			return Summary{}, err
		}
		if err := tx.Commit(); err != nil {
			return Summary{}, err
		}
		return r.Summary(ctx, userID)
	}
	if err := r.updateExplicitProfile(ctx, userID, input); err != nil {
		return Summary{}, err
	}
	return r.Summary(ctx, userID)
}

func (r *MySQLRepository) UpdateExplicitPreferences(ctx context.Context, userID int64, input UpdatePreferencesInput) (Summary, error) {
	if r == nil || r.ext == nil {
		return Summary{}, errors.New("profile repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Summary{}, err
		}
		txRepo := &MySQLRepository{ext: tx}
		if err := txRepo.updateExplicitPreferences(ctx, userID, input); err != nil {
			_ = tx.Rollback()
			return Summary{}, err
		}
		if err := tx.Commit(); err != nil {
			return Summary{}, err
		}
		return r.Summary(ctx, userID)
	}
	if err := r.updateExplicitPreferences(ctx, userID, input); err != nil {
		return Summary{}, err
	}
	return r.Summary(ctx, userID)
}

func (r *MySQLRepository) updateExplicitProfile(ctx context.Context, userID int64, input UpdateProfileInput) error {
	if err := r.ensureUserExistsForUpdate(ctx, userID); err != nil {
		return err
	}
	if input.Nickname.Present {
		if _, err := r.ext.ExecContext(ctx, `UPDATE users SET nickname = NULLIF(?, '') WHERE id = ? AND deleted_at IS NULL`, input.Nickname.Value, userID); err != nil {
			return err
		}
	}
	if !hasProfileFieldPatch(input) {
		return nil
	}
	profile, err := r.ensureActiveProfile(ctx, userID)
	if err != nil {
		return err
	}
	if err := r.updateExplicitProfileFields(ctx, userID, input); err != nil {
		return err
	}
	facts := explicitProfileFacts(input)
	factKeys := explicitProfileFactKeys(input)
	if len(factKeys) == 0 {
		return nil
	}
	return r.ReplaceFactKeys(ctx, userID, profile.ID, factKeys, facts)
}

func (r *MySQLRepository) updateExplicitPreferences(ctx context.Context, userID int64, input UpdatePreferencesInput) error {
	if err := r.ensureUserExistsForUpdate(ctx, userID); err != nil {
		return err
	}
	profile, err := r.ensureActiveProfile(ctx, userID)
	if err != nil {
		return err
	}
	return r.ReplacePrefs(ctx, userID, profile.ID, explicitPreferences(input))
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
  (public_id, user_id, status, gender, height_cm, weight_kg, body_notes, skin_notes, hair_notes, face_shape, upper_body_notes, lower_body_notes, size_notes, lifestyle_scenarios, style_goal_summary)
VALUES
  (?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), NULLIF(?, ''))
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  gender = VALUES(gender),
  height_cm = VALUES(height_cm),
  weight_kg = VALUES(weight_kg),
  body_notes = VALUES(body_notes),
  skin_notes = VALUES(skin_notes),
  hair_notes = VALUES(hair_notes),
  face_shape = VALUES(face_shape),
  upper_body_notes = VALUES(upper_body_notes),
  lower_body_notes = VALUES(lower_body_notes),
  size_notes = VALUES(size_notes),
  lifestyle_scenarios = VALUES(lifestyle_scenarios),
  style_goal_summary = VALUES(style_goal_summary)
`, item.PublicID, item.UserID, item.Status, item.Gender, item.HeightCM, item.WeightKG, item.BodyNotes, item.SkinNotes, item.HairNotes, item.FaceShape, item.UpperBodyNotes, item.LowerBodyNotes, item.SizeNotes, scenarios, item.StyleGoalSummary)
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
  weight_kg,
  COALESCE(body_notes, '') AS body_notes,
  COALESCE(skin_notes, '') AS skin_notes,
  COALESCE(hair_notes, '') AS hair_notes,
  COALESCE(face_shape, '') AS face_shape,
  COALESCE(upper_body_notes, '') AS upper_body_notes,
  COALESCE(lower_body_notes, '') AS lower_body_notes,
  COALESCE(size_notes, '') AS size_notes,
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

func (r *MySQLRepository) upsertExplicitProfile(ctx context.Context, item Profile) (Profile, error) {
	scenarios, err := jsonText(item.LifestyleScenarios)
	if err != nil {
		return Profile{}, err
	}
	_, err = r.ext.ExecContext(ctx, `
INSERT INTO profiles
  (public_id, user_id, status, gender, height_cm, weight_kg, body_notes, skin_notes, hair_notes, face_shape, upper_body_notes, lower_body_notes, size_notes, lifestyle_scenarios)
VALUES
  (?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON))
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  gender = VALUES(gender),
  height_cm = VALUES(height_cm),
  weight_kg = VALUES(weight_kg),
  body_notes = VALUES(body_notes),
  skin_notes = VALUES(skin_notes),
  hair_notes = VALUES(hair_notes),
  face_shape = VALUES(face_shape),
  upper_body_notes = VALUES(upper_body_notes),
  lower_body_notes = VALUES(lower_body_notes),
  size_notes = VALUES(size_notes),
  lifestyle_scenarios = VALUES(lifestyle_scenarios),
  deleted_at = NULL
`, item.PublicID, item.UserID, item.Status, item.Gender, item.HeightCM, item.WeightKG, item.BodyNotes, item.SkinNotes, item.HairNotes, item.FaceShape, item.UpperBodyNotes, item.LowerBodyNotes, item.SizeNotes, scenarios)
	if err != nil {
		return Profile{}, err
	}
	saved, err := r.findProfileByUserID(ctx, item.UserID)
	if err != nil {
		return Profile{}, err
	}
	saved.LifestyleScenarios = item.LifestyleScenarios
	return saved, nil
}

func (r *MySQLRepository) ensureActiveProfile(ctx context.Context, userID int64) (Profile, error) {
	_, err := r.ext.ExecContext(ctx, `
INSERT INTO profiles
  (public_id, user_id, status)
VALUES
  (?, ?, ?)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  deleted_at = NULL
`, id.NewPublicID("prf"), userID, StatusActive)
	if err != nil {
		return Profile{}, err
	}
	return r.findProfileByUserID(ctx, userID)
}

func (r *MySQLRepository) ensureUserExistsForUpdate(ctx context.Context, userID int64) error {
	var id int64
	err := sqlx.GetContext(ctx, r.ext, &id, `SELECT id FROM users WHERE id = ? AND deleted_at IS NULL FOR UPDATE`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUserNotFound
	}
	return err
}

func (r *MySQLRepository) updateExplicitProfileFields(ctx context.Context, userID int64, input UpdateProfileInput) error {
	sets := make([]string, 0, 6)
	args := make([]any, 0, 7)
	if input.Gender.Present {
		sets = append(sets, "gender = NULLIF(?, '')")
		args = append(args, input.Gender.Value)
	}
	if input.HeightCM.Present {
		sets = append(sets, "height_cm = ?")
		args = append(args, input.HeightCM.Value)
	}
	if input.WeightKG.Present {
		sets = append(sets, "weight_kg = ?")
		args = append(args, input.WeightKG.Value)
	}
	if input.BodyNotes.Present {
		sets = append(sets, "body_notes = NULLIF(?, '')")
		args = append(args, input.BodyNotes.Value)
	}
	if input.SkinNotes.Present {
		sets = append(sets, "skin_notes = NULLIF(?, '')")
		args = append(args, input.SkinNotes.Value)
	}
	if input.HairNotes.Present {
		sets = append(sets, "hair_notes = NULLIF(?, '')")
		args = append(args, input.HairNotes.Value)
	}
	if input.FaceShape.Present {
		sets = append(sets, "face_shape = NULLIF(?, '')")
		args = append(args, input.FaceShape.Value)
	}
	if input.UpperBodyNotes.Present {
		sets = append(sets, "upper_body_notes = NULLIF(?, '')")
		args = append(args, input.UpperBodyNotes.Value)
	}
	if input.LowerBodyNotes.Present {
		sets = append(sets, "lower_body_notes = NULLIF(?, '')")
		args = append(args, input.LowerBodyNotes.Value)
	}
	if input.SizeNotes.Present {
		sets = append(sets, "size_notes = NULLIF(?, '')")
		args = append(args, input.SizeNotes.Value)
	}
	if input.LifestyleScenarios.Present {
		scenarios, err := jsonText(input.LifestyleScenarios.Value)
		if err != nil {
			return err
		}
		sets = append(sets, "lifestyle_scenarios = CAST(? AS JSON)")
		args = append(args, scenarios)
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, userID)
	_, err := r.ext.ExecContext(ctx, `
UPDATE profiles
SET `+strings.Join(sets, ", ")+`
WHERE user_id = ? AND deleted_at IS NULL
`, args...)
	return err
}

func (r *MySQLRepository) findProfileByUserID(ctx context.Context, userID int64) (Profile, error) {
	var saved Profile
	err := sqlx.GetContext(ctx, r.ext, &saved, `
SELECT
  id,
  public_id,
  user_id,
  status,
  COALESCE(gender, '') AS gender,
  height_cm,
  weight_kg,
  COALESCE(body_notes, '') AS body_notes,
  COALESCE(skin_notes, '') AS skin_notes,
  COALESCE(hair_notes, '') AS hair_notes,
  COALESCE(face_shape, '') AS face_shape,
  COALESCE(upper_body_notes, '') AS upper_body_notes,
  COALESCE(lower_body_notes, '') AS lower_body_notes,
  COALESCE(size_notes, '') AS size_notes,
  COALESCE(style_goal_summary, '') AS style_goal_summary
FROM profiles
WHERE user_id = ? AND deleted_at IS NULL
LIMIT 1
`, userID)
	return saved, err
}

func (r *MySQLRepository) CreateProfilePhoto(ctx context.Context, userID int64, input CreateProfilePhotoInput) (ProfilePhoto, error) {
	if r == nil || r.ext == nil {
		return ProfilePhoto{}, errors.New("profile repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return ProfilePhoto{}, err
		}
		txRepo := &MySQLRepository{ext: tx}
		photo, err := txRepo.createProfilePhoto(ctx, userID, input)
		if err != nil {
			_ = tx.Rollback()
			return ProfilePhoto{}, err
		}
		if err := tx.Commit(); err != nil {
			return ProfilePhoto{}, err
		}
		return photo, nil
	}
	return r.createProfilePhoto(ctx, userID, input)
}

func (r *MySQLRepository) createProfilePhoto(ctx context.Context, userID int64, input CreateProfilePhotoInput) (ProfilePhoto, error) {
	if err := r.ensureUserExistsForUpdate(ctx, userID); err != nil {
		return ProfilePhoto{}, err
	}
	profile, err := r.ensureActiveProfile(ctx, userID)
	if err != nil {
		return ProfilePhoto{}, err
	}
	if err := r.requireProfilePhotoCapacity(ctx, userID, input.PhotoType); err != nil {
		return ProfilePhoto{}, err
	}
	asset, err := r.findProfilePhotoAsset(ctx, userID, input.AssetPublicID)
	if err != nil {
		return ProfilePhoto{}, err
	}
	publicID := id.NewPublicID("pph")
	_, err = r.ext.ExecContext(ctx, `
INSERT INTO profile_photos
  (public_id, user_id, profile_id, asset_public_id, photo_type, angle, note, sort_order, status)
VALUES
  (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)
`, publicID, userID, profile.ID, asset.PublicID, input.PhotoType, input.Angle, input.Note, input.SortOrder, StatusActive)
	if err != nil {
		return ProfilePhoto{}, err
	}
	return r.findProfilePhoto(ctx, userID, publicID)
}

func (r *MySQLRepository) UpdateProfilePhoto(ctx context.Context, userID int64, publicID string, input UpdateProfilePhotoInput) (ProfilePhoto, error) {
	if r == nil || r.ext == nil {
		return ProfilePhoto{}, errors.New("profile repository database is nil")
	}
	if input.PhotoType.Present {
		currentType, err := r.findProfilePhotoType(ctx, userID, publicID)
		if err != nil {
			return ProfilePhoto{}, err
		}
		if currentType != input.PhotoType.Value {
			if err := r.requireProfilePhotoCapacity(ctx, userID, input.PhotoType.Value); err != nil {
				return ProfilePhoto{}, err
			}
		}
	}
	sets := make([]string, 0, 5)
	args := make([]any, 0, 7)
	if input.PhotoType.Present {
		sets = append(sets, "photo_type = ?")
		args = append(args, input.PhotoType.Value)
	}
	if input.Angle.Present {
		sets = append(sets, "angle = ?")
		args = append(args, input.Angle.Value)
	}
	if input.Note.Present {
		sets = append(sets, "note = NULLIF(?, '')")
		args = append(args, input.Note.Value)
	}
	if input.SortOrder.Present {
		sets = append(sets, "sort_order = ?")
		if input.SortOrder.Value == nil {
			args = append(args, 0)
		} else {
			args = append(args, *input.SortOrder.Value)
		}
	}
	if input.Status.Present {
		sets = append(sets, "status = ?")
		args = append(args, input.Status.Value)
	}
	if len(sets) > 0 {
		args = append(args, userID, strings.TrimSpace(publicID))
		result, err := r.ext.ExecContext(ctx, `
UPDATE profile_photos
SET `+strings.Join(sets, ", ")+`
WHERE user_id = ? AND public_id = ? AND deleted_at IS NULL
`, args...)
		if err != nil {
			return ProfilePhoto{}, err
		}
		if err := dbutil.RequireRowsAffected(result, "profile photo update"); err != nil {
			return ProfilePhoto{}, ErrProfilePhotoNotFound
		}
	}
	return r.findProfilePhoto(ctx, userID, publicID)
}

func (r *MySQLRepository) findProfilePhotoType(ctx context.Context, userID int64, publicID string) (string, error) {
	var photoType string
	err := sqlx.GetContext(ctx, r.ext, &photoType, `
SELECT photo_type
FROM profile_photos
WHERE user_id = ?
  AND public_id = ?
  AND deleted_at IS NULL
LIMIT 1
`, userID, strings.TrimSpace(publicID))
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrProfilePhotoNotFound
	}
	return photoType, err
}

func (r *MySQLRepository) DeleteProfilePhoto(ctx context.Context, userID int64, publicID string) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	result, err := r.ext.ExecContext(ctx, `
UPDATE profile_photos
SET deleted_at = CURRENT_TIMESTAMP(3), status = 'deleted'
WHERE user_id = ? AND public_id = ? AND deleted_at IS NULL
`, userID, strings.TrimSpace(publicID))
	if err != nil {
		return err
	}
	if err := dbutil.RequireRowsAffected(result, "profile photo delete"); err != nil {
		return ErrProfilePhotoNotFound
	}
	return nil
}

type profilePhotoAsset struct {
	PublicID  string `db:"public_id"`
	ObjectKey string `db:"object_key"`
}

func (r *MySQLRepository) findProfilePhotoAsset(ctx context.Context, userID int64, publicID string) (profilePhotoAsset, error) {
	var asset profilePhotoAsset
	err := sqlx.GetContext(ctx, r.ext, &asset, `
SELECT public_id, object_key
FROM files
WHERE public_id = ?
  AND owner_user_id = ?
  AND asset_type = 'profile_photo'
  AND status <> 'deleted'
  AND deleted_at IS NULL
LIMIT 1
`, strings.TrimSpace(publicID), userID)
	if errors.Is(err, sql.ErrNoRows) {
		return profilePhotoAsset{}, ErrProfileAssetNotFound
	}
	return asset, err
}

func (r *MySQLRepository) requireProfilePhotoCapacity(ctx context.Context, userID int64, photoType string) error {
	var counts struct {
		Total int `db:"total"`
		Group int `db:"group_count"`
	}
	err := sqlx.GetContext(ctx, r.ext, &counts, `
SELECT
  COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN photo_type = ? THEN 1 ELSE 0 END), 0) AS group_count
FROM profile_photos
WHERE user_id = ?
  AND deleted_at IS NULL
`, photoType, userID)
	if err != nil {
		return err
	}
	if counts.Total >= maxProfilePhotoCount || counts.Group >= maxPhotoGroupCount {
		return ErrProfilePhotoLimit
	}
	return nil
}

func (r *MySQLRepository) profilePhotos(ctx context.Context, userID int64) ([]ProfilePhoto, error) {
	var rows []profilePhotoRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, profilePhotoSelectSQL(`
WHERE pp.user_id = ?
  AND pp.deleted_at IS NULL
ORDER BY pp.photo_type ASC, pp.sort_order ASC, pp.id ASC
`), userID); err != nil {
		return nil, err
	}
	return profilePhotoRows(rows), nil
}

func (r *MySQLRepository) findProfilePhoto(ctx context.Context, userID int64, publicID string) (ProfilePhoto, error) {
	var row profilePhotoRow
	err := sqlx.GetContext(ctx, r.ext, &row, profilePhotoSelectSQL(`
WHERE pp.user_id = ?
  AND pp.public_id = ?
  AND pp.deleted_at IS NULL
LIMIT 1
`), userID, strings.TrimSpace(publicID))
	if errors.Is(err, sql.ErrNoRows) {
		return ProfilePhoto{}, ErrProfilePhotoNotFound
	}
	if err != nil {
		return ProfilePhoto{}, err
	}
	return row.profilePhoto(), nil
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

func (r *MySQLRepository) ReplaceFactKeys(ctx context.Context, userID int64, profileID int64, factKeys []string, facts []Fact) error {
	if r == nil || r.ext == nil {
		return errors.New("profile repository database is nil")
	}
	if len(factKeys) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(factKeys)), ",")
	args := make([]any, 0, 2+len(factKeys))
	args = append(args, userID, profileID)
	for _, key := range factKeys {
		args = append(args, key)
	}
	if _, err := r.ext.ExecContext(ctx, `UPDATE profile_facts SET deleted_at = CURRENT_TIMESTAMP(3) WHERE user_id = ? AND profile_id = ? AND deleted_at IS NULL AND fact_key IN (`+placeholders+`)`, args...); err != nil {
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

func (r *MySQLRepository) preferencesSummary(ctx context.Context, userID int64) (PreferencesSummary, error) {
	var rows []preferenceSummaryRow
	if err := sqlx.SelectContext(ctx, r.ext, &rows, `
SELECT
  pref_type,
  pref_key,
  polarity
FROM profile_prefs
WHERE user_id = ?
  AND deleted_at IS NULL
ORDER BY id ASC
`, userID); err != nil {
		return PreferencesSummary{}, err
	}
	summary := PreferencesSummary{
		StyleGoals:          []string{},
		Avoidances:          []string{},
		ScenarioPreferences: []string{},
	}
	for _, row := range rows {
		switch row.PrefType {
		case PrefTypeStyleGoal:
			summary.StyleGoals = append(summary.StyleGoals, row.PrefKey)
		case PrefTypeAvoidance:
			summary.Avoidances = append(summary.Avoidances, row.PrefKey)
		case PrefTypeScenarioPreference:
			summary.ScenarioPreferences = append(summary.ScenarioPreferences, row.PrefKey)
		default:
			if row.Polarity == PolarityNegative {
				summary.Avoidances = append(summary.Avoidances, row.PrefKey)
			}
		}
	}
	return summary, nil
}

type profileSummaryRow struct {
	UserPublicID       string          `db:"user_public_id"`
	Nickname           string          `db:"nickname"`
	OnboardingStatus   string          `db:"onboarding_status"`
	ProfileID          sql.NullInt64   `db:"profile_id"`
	ProfilePublicID    sql.NullString  `db:"profile_public_id"`
	Gender             sql.NullString  `db:"gender"`
	HeightCM           sql.NullInt64   `db:"height_cm"`
	WeightKG           sql.NullFloat64 `db:"weight_kg"`
	BodyNotes          sql.NullString  `db:"body_notes"`
	SkinNotes          sql.NullString  `db:"skin_notes"`
	HairNotes          sql.NullString  `db:"hair_notes"`
	FaceShape          sql.NullString  `db:"face_shape"`
	UpperBodyNotes     sql.NullString  `db:"upper_body_notes"`
	LowerBodyNotes     sql.NullString  `db:"lower_body_notes"`
	SizeNotes          sql.NullString  `db:"size_notes"`
	LifestyleScenarios sql.NullString  `db:"lifestyle_scenarios"`
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
	scenarios, err := decodeStringSlice(json.RawMessage(r.LifestyleScenarios.String))
	if err != nil {
		return Summary{}, err
	}
	profile := &ProfileSummary{
		ProfilePublicID:    r.ProfilePublicID.String,
		Gender:             r.Gender.String,
		BodyNotes:          r.BodyNotes.String,
		SkinNotes:          r.SkinNotes.String,
		HairNotes:          r.HairNotes.String,
		FaceShape:          r.FaceShape.String,
		UpperBodyNotes:     r.UpperBodyNotes.String,
		LowerBodyNotes:     r.LowerBodyNotes.String,
		SizeNotes:          r.SizeNotes.String,
		LifestyleScenarios: scenarios,
		StyleGoalSummary:   r.StyleGoalSummary.String,
	}
	if r.HeightCM.Valid {
		height := int(r.HeightCM.Int64)
		profile.HeightCM = &height
	}
	if r.WeightKG.Valid {
		weight := r.WeightKG.Float64
		profile.WeightKG = &weight
	}
	summary.Profile = profile
	return summary, nil
}

type profilePhotoRow struct {
	PublicID      string         `db:"public_id"`
	AssetPublicID string         `db:"asset_public_id"`
	PhotoType     string         `db:"photo_type"`
	Angle         string         `db:"angle"`
	Note          sql.NullString `db:"note"`
	SortOrder     int            `db:"sort_order"`
	Status        string         `db:"status"`
	ObjectKey     sql.NullString `db:"object_key"`
}

func (r profilePhotoRow) profilePhoto() ProfilePhoto {
	photo := ProfilePhoto{
		PublicID:      r.PublicID,
		AssetPublicID: r.AssetPublicID,
		PhotoType:     r.PhotoType,
		Angle:         r.Angle,
		Note:          r.Note.String,
		SortOrder:     r.SortOrder,
		Status:        r.Status,
	}
	if r.ObjectKey.Valid && r.ObjectKey.String != "" {
		photo.Image = &ProfilePhotoImage{ObjectKey: r.ObjectKey.String}
	}
	return photo
}

func profilePhotoRows(rows []profilePhotoRow) []ProfilePhoto {
	photos := make([]ProfilePhoto, 0, len(rows))
	for _, row := range rows {
		photos = append(photos, row.profilePhoto())
	}
	return photos
}

func profilePhotoSelectSQL(where string) string {
	return `
SELECT
  pp.public_id,
  pp.asset_public_id,
  pp.photo_type,
  pp.angle,
  pp.note,
  pp.sort_order,
  pp.status,
  f.object_key
FROM profile_photos pp
LEFT JOIN files f
  ON f.public_id = pp.asset_public_id
  AND f.owner_user_id = pp.user_id
  AND f.deleted_at IS NULL
` + where
}

type latestReportSummaryRow struct {
	PublicID    string       `db:"public_id"`
	Title       string       `db:"title"`
	Status      string       `db:"status"`
	GeneratedAt sql.NullTime `db:"generated_at"`
}

type preferenceSummaryRow struct {
	PrefType string `db:"pref_type"`
	PrefKey  string `db:"pref_key"`
	Polarity string `db:"polarity"`
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

func explicitProfileFacts(input UpdateProfileInput) []Fact {
	facts := make([]Fact, 0, 3)
	if input.Gender.Present && input.Gender.Value != "" {
		facts = append(facts, Fact{Key: "gender", Value: input.Gender.Value, Source: SourceUser})
	}
	if input.HeightCM.Present && input.HeightCM.Value != nil {
		facts = append(facts, Fact{Key: "height_cm", Value: input.HeightCM.Value, Source: SourceUser})
	}
	if input.WeightKG.Present && input.WeightKG.Value != nil {
		facts = append(facts, Fact{Key: "weight_kg", Value: input.WeightKG.Value, Source: SourceUser})
	}
	if input.FaceShape.Present && input.FaceShape.Value != "" {
		facts = append(facts, Fact{Key: "face_shape", Value: input.FaceShape.Value, Source: SourceUser})
	}
	if input.LifestyleScenarios.Present {
		facts = append(facts, Fact{Key: "lifestyle_scenarios", Value: input.LifestyleScenarios.Value, Source: SourceUser})
	}
	return facts
}

func explicitProfileFactKeys(input UpdateProfileInput) []string {
	keys := make([]string, 0, 3)
	if input.Gender.Present {
		keys = append(keys, "gender")
	}
	if input.HeightCM.Present {
		keys = append(keys, "height_cm")
	}
	if input.WeightKG.Present {
		keys = append(keys, "weight_kg")
	}
	if input.FaceShape.Present {
		keys = append(keys, "face_shape")
	}
	if input.LifestyleScenarios.Present {
		keys = append(keys, "lifestyle_scenarios")
	}
	return keys
}

func hasProfileFieldPatch(input UpdateProfileInput) bool {
	return input.Gender.Present ||
		input.HeightCM.Present ||
		input.WeightKG.Present ||
		input.BodyNotes.Present ||
		input.SkinNotes.Present ||
		input.HairNotes.Present ||
		input.FaceShape.Present ||
		input.UpperBodyNotes.Present ||
		input.LowerBodyNotes.Present ||
		input.SizeNotes.Present ||
		input.LifestyleScenarios.Present
}

func explicitPreferences(input UpdatePreferencesInput) []Pref {
	prefs := make([]Pref, 0, len(input.StyleGoals)+len(input.Avoidances)+len(input.ScenarioPreferences))
	for _, goal := range input.StyleGoals {
		prefs = append(prefs, Pref{Type: PrefTypeStyleGoal, Key: goal, Value: goal, Polarity: PolarityPositive, Source: SourceUser})
	}
	for _, avoidance := range input.Avoidances {
		prefs = append(prefs, Pref{Type: PrefTypeAvoidance, Key: avoidance, Value: avoidance, Polarity: PolarityNegative, Source: SourceUser})
	}
	for _, scenario := range input.ScenarioPreferences {
		prefs = append(prefs, Pref{Type: PrefTypeScenarioPreference, Key: scenario, Value: scenario, Polarity: PolarityPositive, Source: SourceUser})
	}
	return prefs
}

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}
