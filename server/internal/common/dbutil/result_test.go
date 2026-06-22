package dbutil

import (
	"database/sql"
	"errors"
	"testing"
)

type fakeResult struct {
	lastInsertID int64
	rowsAffected int64
	lastErr      error
	rowsErr      error
}

func (r fakeResult) LastInsertId() (int64, error) {
	return r.lastInsertID, r.lastErr
}

func (r fakeResult) RowsAffected() (int64, error) {
	return r.rowsAffected, r.rowsErr
}

func TestRequireLastInsertIDReturnsDriverError(t *testing.T) {
	expected := errors.New("driver does not support last insert id")

	_, err := RequireLastInsertID(fakeResult{lastErr: expected}, "report create")

	if !errors.Is(err, expected) {
		t.Fatalf("expected driver error, got %v", err)
	}
}

func TestRequireLastInsertIDRejectsZeroID(t *testing.T) {
	_, err := RequireLastInsertID(fakeResult{}, "report create")

	if err == nil {
		t.Fatal("expected error for zero insert id")
	}
}

func TestRequireRowsAffectedRejectsZeroRows(t *testing.T) {
	err := RequireRowsAffected(fakeResult{rowsAffected: 0}, "report mark ready")

	if err == nil {
		t.Fatal("expected error for zero affected rows")
	}
}

func TestRequireRowsAffectedAcceptsAffectedRows(t *testing.T) {
	err := RequireRowsAffected(fakeResult{rowsAffected: 1}, "report mark ready")

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

var _ sql.Result = fakeResult{}
