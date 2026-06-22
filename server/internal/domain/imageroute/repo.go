package imageroute

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

func (r *MySQLRepository) FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Route, error) {
	if r == nil || r.ext == nil {
		return Route{}, errors.New("image route repository database is nil")
	}
	var row routeRow
	err := sqlx.GetContext(ctx, r.ext, &row, `
SELECT
  id,
  public_id,
  user_id,
  profile_id,
  status,
  activated_at
FROM image_routes
WHERE public_id = ?
  AND user_id = ?
  AND deleted_at IS NULL
LIMIT 1
`, publicID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Route{}, ErrRouteNotFound
	}
	if err != nil {
		return Route{}, err
	}
	return row.toRoute(), nil
}

func (r *MySQLRepository) UpdateFeedback(ctx context.Context, route Route, event Event) (Route, error) {
	if r == nil || r.ext == nil {
		return Route{}, errors.New("image route repository database is nil")
	}
	if starter, ok := r.ext.(txStarter); ok {
		tx, err := starter.BeginTxx(ctx, nil)
		if err != nil {
			return Route{}, err
		}
		updated, err := (&MySQLRepository{ext: tx}).updateFeedback(ctx, route, event)
		if err != nil {
			_ = tx.Rollback()
			return Route{}, err
		}
		if err := tx.Commit(); err != nil {
			return Route{}, err
		}
		return updated, nil
	}
	return r.updateFeedback(ctx, route, event)
}

func (r *MySQLRepository) updateFeedback(ctx context.Context, route Route, event Event) (Route, error) {
	result, err := r.ext.ExecContext(ctx, `
UPDATE image_routes
SET status = ?, activated_at = ?
WHERE id = ?
  AND user_id = ?
  AND deleted_at IS NULL
`, route.Status, route.ActivatedAt, route.ID, route.UserID)
	if err != nil {
		return Route{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return Route{}, fmt.Errorf("image route update feedback rows affected: %w", err)
	}
	if rows == 0 {
		return Route{}, ErrRouteNotFound
	}
	value, err := jsonText(event.EventValue)
	if err != nil {
		return Route{}, err
	}
	result, err = r.ext.ExecContext(ctx, `
INSERT INTO image_route_events
  (image_route_id, user_id, event_type, event_value, source, created_by)
VALUES
  (?, ?, ?, CAST(? AS JSON), ?, ?)
`, event.ImageRouteID, event.UserID, event.EventType, value, event.Source, event.CreatedBy)
	if err != nil {
		return Route{}, err
	}
	if err := dbutil.RequireRowsAffected(result, "image route feedback event create"); err != nil {
		return Route{}, err
	}
	return route, nil
}

type txStarter interface {
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
}

type routeRow struct {
	ID          int64        `db:"id"`
	PublicID    string       `db:"public_id"`
	UserID      int64        `db:"user_id"`
	ProfileID   int64        `db:"profile_id"`
	Status      string       `db:"status"`
	ActivatedAt sql.NullTime `db:"activated_at"`
}

func (r routeRow) toRoute() Route {
	var activatedAt *time.Time
	if r.ActivatedAt.Valid {
		activatedAt = &r.ActivatedAt.Time
	}
	return Route{
		ID:          r.ID,
		PublicID:    r.PublicID,
		UserID:      r.UserID,
		ProfileID:   r.ProfileID,
		Status:      r.Status,
		ActivatedAt: activatedAt,
	}
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
