package imageroute

import (
	"context"
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

func (r *MySQLRepository) CreateMany(ctx context.Context, items []Route) ([]Route, error) {
	if r == nil || r.ext == nil {
		return nil, errors.New("image route repository database is nil")
	}
	for i := range items {
		target, err := jsonText(items[i].TargetImpression)
		if err != nil {
			return nil, err
		}
		scenes, err := jsonText(items[i].SuitableScenes)
		if err != nil {
			return nil, err
		}
		hair, err := jsonText(items[i].HairStrategy)
		if err != nil {
			return nil, err
		}
		makeup, err := jsonText(items[i].MakeupStrategy)
		if err != nil {
			return nil, err
		}
		outfit, err := jsonText(items[i].OutfitStrategy)
		if err != nil {
			return nil, err
		}
		avoid, err := jsonText(items[i].AvoidPoints)
		if err != nil {
			return nil, err
		}
		reason, err := jsonText(items[i].Reason)
		if err != nil {
			return nil, err
		}
		result, err := r.ext.ExecContext(ctx, `
INSERT INTO image_routes
  (public_id, user_id, profile_id, name, route_role, status, weight, target_impression, suitable_scenes, hair_strategy, makeup_strategy, outfit_strategy, avoid_points, reason, source, created_from_report_id, created_from_job_id)
VALUES
  (?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), ?, ?, ?)
`, items[i].PublicID, items[i].UserID, items[i].ProfileID, items[i].Name, items[i].RouteRole, items[i].Status, items[i].Weight, target, scenes, hair, makeup, outfit, avoid, reason, items[i].Source, items[i].CreatedFromReportID, items[i].CreatedFromJobID)
		if err != nil {
			return nil, err
		}
		id, err := dbutil.RequireLastInsertID(result, "image route create")
		if err != nil {
			return nil, err
		}
		items[i].ID = id
	}
	return items, nil
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
