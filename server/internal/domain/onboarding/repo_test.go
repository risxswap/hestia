package onboarding

import (
	"database/sql"
	"errors"
	"testing"
)

type fakeSQLResult struct {
	rowsAffected int64
	err          error
}

func (r fakeSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeSQLResult) RowsAffected() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.rowsAffected, nil
}

func TestRowsAffectedErrorReturnsDraftNotFoundWhenUpdateTouchesNoRows(t *testing.T) {
	err := rowsAffectedError(fakeSQLResult{rowsAffected: 0})

	if !errors.Is(err, ErrDraftNotFound) {
		t.Fatalf("expected ErrDraftNotFound, got %v", err)
	}
}

func TestRowsAffectedErrorReturnsUnderlyingRowsAffectedError(t *testing.T) {
	expected := errors.New("driver error")

	err := rowsAffectedError(fakeSQLResult{err: expected})

	if !errors.Is(err, expected) {
		t.Fatalf("expected underlying error, got %v", err)
	}
}

func TestRowsAffectedErrorAcceptsUpdatedRows(t *testing.T) {
	err := rowsAffectedError(fakeSQLResult{rowsAffected: 1})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

var _ sql.Result = fakeSQLResult{}
