package migration

import (
	"context"
	"database/sql"
	"errors"
)

var defaultDataCorrections = []DataCorrection{
	{
		Key:         "clothes_item_options_string_array_20260704",
		Type:        CorrectionTypeSQL,
		Description: "将衣服下拉建议配置订正为中文字符串数组",
		SQL: []string{
			`UPDATE system_configs
SET value = CAST('["上装","下装","外套","鞋","包","配饰","运动","家居","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'clothes.item_options'
  AND ` + "`key`" + ` = 'categories'`,
			`UPDATE system_configs
SET value = CAST('["黑色","白色","米白","灰色","深蓝","浅蓝","棕色","卡其","红色","绿色","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'clothes.item_options'
  AND ` + "`key`" + ` = 'colors'`,
			`UPDATE system_configs
SET value = CAST('["棉","亚麻","羊毛","针织","牛仔","真丝","皮革","聚酯纤维","混纺","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'clothes.item_options'
  AND ` + "`key`" + ` = 'materials'`,
			`UPDATE system_configs
SET value = CAST('["春夏","春秋","秋冬","夏季","冬季","四季"]' AS JSON)
WHERE ` + "`group`" + ` = 'clothes.item_options'
  AND ` + "`key`" + ` = 'seasons'`,
			`UPDATE system_configs
SET value = CAST('["修身","合身","微宽松","宽松","直筒","A 字","短款","长款","高腰","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'clothes.item_options'
  AND ` + "`key`" + ` = 'silhouettes'`,
		},
	},
	{
		Key:         "legacy_wardrobe_tables_to_clothes_20260705",
		Type:        CorrectionTypeCode,
		Description: "将开发库旧 wardrobe 表数据迁移到 clothes 表",
		Run:         migrateLegacyWardrobeTablesToClothes,
	},
}

func migrateLegacyWardrobeTablesToClothes(ctx context.Context, exec SQLExecutor) error {
	if exists, err := mysqlTableExists(ctx, exec, "wardrobe_items"); err != nil {
		return err
	} else if exists {
		if _, err := exec.ExecContext(ctx, `
INSERT IGNORE INTO clothes
  (id, public_id, user_id, name, category, color, silhouette, material, thickness, pattern, season, formality, scene_tags, ai_attrs, user_notes, is_core, recommendation_status, recognition_status, status, created_at, updated_at, deleted_at)
SELECT
  id, public_id, user_id, name, category, color, silhouette, material, thickness, pattern, season, formality, scene_tags, ai_attrs, user_notes, is_core, recommendation_status, recognition_status, status, created_at, updated_at, deleted_at
FROM wardrobe_items
`); err != nil {
			return err
		}
	}
	if exists, err := mysqlTableExists(ctx, exec, "wardrobe_item_assets"); err != nil {
		return err
	} else if exists {
		if _, err := exec.ExecContext(ctx, `
INSERT IGNORE INTO clothes_assets
  (id, clothes_id, asset_id, is_primary, sort_order, created_at)
SELECT
  id, wardrobe_item_id, asset_id, is_primary, sort_order, created_at
FROM wardrobe_item_assets
`); err != nil {
			return err
		}
	}
	if exists, err := mysqlTableExists(ctx, exec, "wardrobe_gaps"); err != nil {
		return err
	} else if exists {
		if _, err := exec.ExecContext(ctx, `
INSERT IGNORE INTO clothes_gaps
  (id, public_id, user_id, gap_type, title, description, reason, priority, source_report_id, source_advice_id, status, created_at, updated_at, deleted_at)
SELECT
  id, public_id, user_id, gap_type, title, description, reason, priority, source_report_id, source_advice_id, status, created_at, updated_at, deleted_at
FROM wardrobe_gaps
`); err != nil {
			return err
		}
	}
	if _, err := exec.ExecContext(ctx, `
INSERT IGNORE INTO system_configs
  (`+"`group`, `key`, `value`, `value_type`, `description`, `status`"+`)
SELECT
  'clothes.item_options', `+"`key`, `value`, `value_type`, `description`, `status`"+`
FROM system_configs
WHERE `+"`group`"+` = 'wardrobe.item_options'
`); err != nil {
		return err
	}
	return nil
}

func mysqlTableExists(ctx context.Context, exec SQLExecutor, table string) (bool, error) {
	getter, ok := exec.(SQLGetter)
	if !ok {
		return false, errors.New("data correction executor does not support query")
	}
	var count int
	err := getter.GetContext(ctx, &count, `
SELECT COUNT(*)
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_name = ?
`, table)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
