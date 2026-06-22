package dbutil

import (
	"database/sql"
	"fmt"
)

func RequireLastInsertID(result sql.Result, operation string) (int64, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s last insert id: %w", operation, err)
	}
	if id == 0 {
		return 0, fmt.Errorf("%s last insert id is zero", operation)
	}
	return id, nil
}

func RequireRowsAffected(result sql.Result, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s affected zero rows", operation)
	}
	return nil
}
