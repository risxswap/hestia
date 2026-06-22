package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type MySQLRepository struct {
	db *sqlx.DB
}

func NewMySQLRepository(db *sqlx.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Create(ctx context.Context, item Report) (Report, error) {
	if r == nil || r.db == nil {
		return Report{}, errors.New("report repository database is nil")
	}
	content, err := jsonText(item.ContentJSON)
	if err != nil {
		return Report{}, err
	}
	contextSnapshot, err := jsonText(item.ContextSnapshot)
	if err != nil {
		return Report{}, err
	}
	styleRefs, err := jsonText(item.StyleRefsJSON)
	if err != nil {
		return Report{}, err
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO reports
  (public_id, user_id, profile_id, report_type, status, title, summary, content_json, context_snapshot, style_refs_json, job_id, generated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), ?, ?)
`, item.PublicID, item.UserID, item.ProfileID, item.ReportType, item.Status, item.Title, item.Summary, content, contextSnapshot, styleRefs, item.JobID, item.GeneratedAt)
	if err != nil {
		return Report{}, err
	}
	if id, err := result.LastInsertId(); err == nil {
		item.ID = id
	}
	return item, nil
}

func (r *MySQLRepository) AddRoutes(ctx context.Context, reportID int64, routes []ReportRoute) error {
	if r == nil || r.db == nil {
		return errors.New("report repository database is nil")
	}
	for _, route := range routes {
		snapshot, err := jsonText(route.RouteSnapshot)
		if err != nil {
			return err
		}
		if _, err := r.db.ExecContext(ctx, `
INSERT INTO report_image_routes
  (report_id, image_route_id, route_role, sort_order, route_snapshot)
VALUES
  (?, ?, ?, ?, CAST(? AS JSON))
`, reportID, route.ImageRouteID, route.RouteRole, route.SortOrder, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) LatestInitialForUser(ctx context.Context, userID int64) (Report, error) {
	return r.findOne(ctx, `
SELECT
  id,
  public_id,
  user_id,
  profile_id,
  report_type,
  status,
  title,
  COALESCE(summary, '') AS summary,
  COALESCE(content_json, JSON_OBJECT()) AS content_json,
  COALESCE(context_snapshot, JSON_OBJECT()) AS context_snapshot,
  COALESCE(style_refs_json, JSON_OBJECT()) AS style_refs_json,
  COALESCE(job_id, 0) AS job_id,
  generated_at
FROM reports
WHERE user_id = ?
  AND report_type = 'initial'
  AND status = 'ready'
  AND deleted_at IS NULL
ORDER BY generated_at DESC, id DESC
LIMIT 1
`, userID)
}

func (r *MySQLRepository) FindByPublicIDForUser(ctx context.Context, userID int64, publicID string) (Report, error) {
	return r.findOne(ctx, `
SELECT
  id,
  public_id,
  user_id,
  profile_id,
  report_type,
  status,
  title,
  COALESCE(summary, '') AS summary,
  COALESCE(content_json, JSON_OBJECT()) AS content_json,
  COALESCE(context_snapshot, JSON_OBJECT()) AS context_snapshot,
  COALESCE(style_refs_json, JSON_OBJECT()) AS style_refs_json,
  COALESCE(job_id, 0) AS job_id,
  generated_at
FROM reports
WHERE user_id = ?
  AND public_id = ?
  AND deleted_at IS NULL
LIMIT 1
`, userID, publicID)
}

func (r *MySQLRepository) RoutesByReportID(ctx context.Context, reportID int64) ([]ReportRoute, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("report repository database is nil")
	}
	var rows []reportRouteRow
	if err := r.db.SelectContext(ctx, &rows, `
SELECT
  image_route_id,
  route_role,
  sort_order,
  COALESCE(route_snapshot, JSON_OBJECT()) AS route_snapshot
FROM report_image_routes
WHERE report_id = ?
ORDER BY sort_order ASC, id ASC
`, reportID); err != nil {
		return nil, err
	}
	routes := make([]ReportRoute, 0, len(rows))
	for _, row := range rows {
		snapshot, err := decodeJSONMap(row.RouteSnapshot)
		if err != nil {
			return nil, err
		}
		routes = append(routes, ReportRoute{
			ImageRouteID:  row.ImageRouteID,
			RouteRole:     row.RouteRole,
			SortOrder:     row.SortOrder,
			RouteSnapshot: snapshot,
		})
	}
	return routes, nil
}

func (r *MySQLRepository) findOne(ctx context.Context, query string, args ...any) (Report, error) {
	if r == nil || r.db == nil {
		return Report{}, errors.New("report repository database is nil")
	}
	var row reportRow
	err := r.db.GetContext(ctx, &row, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return Report{}, ErrReportNotFound
	}
	if err != nil {
		return Report{}, err
	}
	return row.toReport()
}

type reportRow struct {
	ID              int64           `db:"id"`
	PublicID        string          `db:"public_id"`
	UserID          int64           `db:"user_id"`
	ProfileID       int64           `db:"profile_id"`
	ReportType      string          `db:"report_type"`
	Status          string          `db:"status"`
	Title           string          `db:"title"`
	Summary         string          `db:"summary"`
	ContentJSON     json.RawMessage `db:"content_json"`
	ContextSnapshot json.RawMessage `db:"context_snapshot"`
	StyleRefsJSON   json.RawMessage `db:"style_refs_json"`
	JobID           int64           `db:"job_id"`
	GeneratedAt     sql.NullTime    `db:"generated_at"`
}

func (r reportRow) toReport() (Report, error) {
	content, err := decodeJSONMap(r.ContentJSON)
	if err != nil {
		return Report{}, err
	}
	contextSnapshot, err := decodeJSONMap(r.ContextSnapshot)
	if err != nil {
		return Report{}, err
	}
	styleRefs, err := decodeJSONMap(r.StyleRefsJSON)
	if err != nil {
		return Report{}, err
	}
	var generatedAt *time.Time
	if r.GeneratedAt.Valid {
		generatedAt = &r.GeneratedAt.Time
	}
	return Report{
		ID:              r.ID,
		PublicID:        r.PublicID,
		UserID:          r.UserID,
		ProfileID:       r.ProfileID,
		ReportType:      r.ReportType,
		Status:          r.Status,
		Title:           r.Title,
		Summary:         r.Summary,
		ContentJSON:     content,
		ContextSnapshot: contextSnapshot,
		StyleRefsJSON:   styleRefs,
		JobID:           r.JobID,
		GeneratedAt:     generatedAt,
	}, nil
}

type reportRouteRow struct {
	ImageRouteID  int64           `db:"image_route_id"`
	RouteRole     string          `db:"route_role"`
	SortOrder     int             `db:"sort_order"`
	RouteSnapshot json.RawMessage `db:"route_snapshot"`
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

func decodeJSONMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]any{}
	}
	return data, nil
}
