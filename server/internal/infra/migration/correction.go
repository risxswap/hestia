package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	CorrectionTypeSQL  = "sql"
	CorrectionTypeCode = "code"
)

type DataCorrection struct {
	Key         string
	Type        string
	Description string
	SQL         []string
	Run         func(ctx context.Context, exec SQLExecutor) error
}

func ApplyDataCorrections(ctx context.Context, exec SQLExecutor, corrections []DataCorrection) error {
	for _, correction := range corrections {
		correction.Key = strings.TrimSpace(correction.Key)
		correction.Type = strings.TrimSpace(correction.Type)
		if correction.Key == "" {
			return errors.New("data correction key is required")
		}
		if correction.Type == "" {
			correction.Type = CorrectionTypeSQL
		}
		executed, err := dataCorrectionExecuted(ctx, exec, correction.Key)
		if err != nil {
			return fmt.Errorf("check data correction %s: %w", correction.Key, err)
		}
		if executed {
			continue
		}
		if err := runDataCorrection(ctx, exec, correction); err != nil {
			return fmt.Errorf("run data correction %s: %w", correction.Key, err)
		}
		if err := recordDataCorrection(ctx, exec, correction); err != nil {
			return fmt.Errorf("record data correction %s: %w", correction.Key, err)
		}
	}
	return nil
}

func dataCorrectionExecuted(ctx context.Context, exec SQLExecutor, key string) (bool, error) {
	getter, ok := exec.(SQLGetter)
	if !ok {
		return false, errors.New("data correction executor does not support query")
	}
	var marker int
	err := getter.GetContext(ctx, &marker, "SELECT 1 FROM `data_corrections` WHERE `correction_key` = ? AND `status` = 'succeeded' LIMIT 1", key)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return marker == 1, nil
}

func runDataCorrection(ctx context.Context, exec SQLExecutor, correction DataCorrection) error {
	switch correction.Type {
	case CorrectionTypeSQL:
		for _, statement := range correction.SQL {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := exec.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	case CorrectionTypeCode:
		if correction.Run == nil {
			return errors.New("code data correction runner is required")
		}
		return correction.Run(ctx, exec)
	default:
		return fmt.Errorf("unsupported data correction type %s", correction.Type)
	}
}

func recordDataCorrection(ctx context.Context, exec SQLExecutor, correction DataCorrection) error {
	_, err := exec.ExecContext(ctx, `
INSERT INTO `+"`data_corrections`"+`
  (`+"`correction_key`, `correction_type`, `description`, `checksum`, `status`, `executed_at`"+`)
VALUES
  (?, ?, ?, ?, 'succeeded', CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE
  `+"`correction_type` = VALUES(`correction_type`),"+`
  `+"`description` = VALUES(`description`),"+`
  `+"`checksum` = VALUES(`checksum`),"+`
  `+"`status` = 'succeeded',"+`
  `+"`error_message` = NULL,"+`
  `+"`executed_at` = VALUES(`executed_at`)"+`
`, correction.Key, correction.Type, strings.TrimSpace(correction.Description), dataCorrectionChecksum(correction))
	return err
}

func dataCorrectionChecksum(correction DataCorrection) string {
	sum := sha256.Sum256([]byte(correction.Key + "\n" + correction.Type + "\n" + strings.Join(correction.SQL, "\n")))
	return hex.EncodeToString(sum[:])
}
