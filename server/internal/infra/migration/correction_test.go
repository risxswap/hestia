package migration

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

var errFakeCorrection = errors.New("fake correction failed")

func TestApplyDataCorrectionsSupportsSQLXDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	mock.ExpectQuery("SELECT 1 FROM `data_corrections`").
		WithArgs("fix_sqlx").
		WillReturnRows(sqlmock.NewRows([]string{"1"}))
	mock.ExpectExec("UPDATE `files`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO `data_corrections`").
		WithArgs("fix_sqlx", CorrectionTypeSQL, "sqlx db 订正", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = ApplyDataCorrections(context.Background(), sqlxDB, []DataCorrection{
		{
			Key:         "fix_sqlx",
			Type:        CorrectionTypeSQL,
			Description: "sqlx db 订正",
			SQL:         []string{"UPDATE `files` SET `review_status` = 'pending' WHERE `review_status` = ''"},
		},
	})
	if err != nil {
		t.Fatalf("apply data corrections with sqlx db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestApplyDataCorrectionsRunsOnlyPendingSQLAndCodeCorrections(t *testing.T) {
	exec := newFakeCorrectionExecutor()
	exec.executed["already_done"] = true
	var codeCalls int

	err := ApplyDataCorrections(context.Background(), exec, []DataCorrection{
		{
			Key:         "already_done",
			Type:        CorrectionTypeSQL,
			Description: "已执行订正",
			SQL:         []string{"UPDATE skipped SET value = 1"},
		},
		{
			Key:         "fix_sql",
			Type:        CorrectionTypeSQL,
			Description: "SQL 订正",
			SQL:         []string{"UPDATE users SET status = 'active' WHERE status = ''"},
		},
		{
			Key:         "fix_code",
			Type:        CorrectionTypeCode,
			Description: "代码订正",
			Run: func(ctx context.Context, exec SQLExecutor) error {
				codeCalls++
				_, err := exec.ExecContext(ctx, "UPDATE files SET status = 'active' WHERE status = ''")
				return err
			},
		},
	})
	if err != nil {
		t.Fatalf("apply data corrections: %v", err)
	}

	if containsStatement(exec.queries, "UPDATE skipped") {
		t.Fatalf("expected already executed correction to be skipped, queries=%#v", exec.queries)
	}
	if !containsStatement(exec.queries, "UPDATE users SET status = 'active'") {
		t.Fatalf("expected pending sql correction to run, queries=%#v", exec.queries)
	}
	if !containsStatement(exec.queries, "UPDATE files SET status = 'active'") || codeCalls != 1 {
		t.Fatalf("expected pending code correction to run once, calls=%d queries=%#v", codeCalls, exec.queries)
	}
	if exec.executed["fix_sql"] != true || exec.executed["fix_code"] != true {
		t.Fatalf("expected corrections to be recorded, executed=%#v", exec.executed)
	}
}

func TestApplyDataCorrectionsStopsAndDoesNotRecordFailedCorrection(t *testing.T) {
	exec := newFakeCorrectionExecutor()
	exec.failOn = "UPDATE broken"

	err := ApplyDataCorrections(context.Background(), exec, []DataCorrection{
		{
			Key:  "broken_sql",
			Type: CorrectionTypeSQL,
			SQL:  []string{"UPDATE broken SET value = 1"},
		},
	})
	if err == nil {
		t.Fatal("expected failed correction error")
	}
	if exec.executed["broken_sql"] {
		t.Fatalf("failed correction must not be recorded as executed")
	}
}

type fakeCorrectionExecutor struct {
	queries  []string
	executed map[string]bool
	failOn   string
}

func newFakeCorrectionExecutor() *fakeCorrectionExecutor {
	return &fakeCorrectionExecutor{executed: map[string]bool{}}
}

func (f *fakeCorrectionExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	f.queries = append(f.queries, query)
	normalized := strings.Join(strings.Fields(query), " ")
	if strings.Contains(query, "INSERT INTO `data_corrections`") {
		key := args[0].(string)
		f.executed[key] = true
		return fakeResult{rowsAffected: 1}, nil
	}
	if f.failOn != "" && strings.Contains(normalized, f.failOn) {
		return nil, errFakeCorrection
	}
	return fakeResult{rowsAffected: 1}, nil
}

func (f *fakeCorrectionExecutor) GetContext(_ context.Context, dest any, query string, args ...any) error {
	f.queries = append(f.queries, query)
	if strings.Contains(query, "SELECT 1 FROM `data_corrections`") {
		key := args[0].(string)
		if !f.executed[key] {
			return sql.ErrNoRows
		}
		value, ok := dest.(*int)
		if ok {
			*value = 1
		}
		return nil
	}
	return sql.ErrNoRows
}

type fakeResult struct {
	rowsAffected int64
}

func (r fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
