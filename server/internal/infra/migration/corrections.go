package migration

var defaultDataCorrections = []DataCorrection{
	{
		Key:         "wardrobe_item_options_string_array_20260704",
		Type:        CorrectionTypeSQL,
		Description: "将衣橱下拉建议配置订正为中文字符串数组",
		SQL: []string{
			`UPDATE system_configs
SET value = CAST('["上装","下装","外套","鞋","包","配饰","运动","家居","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'wardrobe.item_options'
  AND ` + "`key`" + ` = 'categories'`,
			`UPDATE system_configs
SET value = CAST('["黑色","白色","米白","灰色","深蓝","浅蓝","棕色","卡其","红色","绿色","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'wardrobe.item_options'
  AND ` + "`key`" + ` = 'colors'`,
			`UPDATE system_configs
SET value = CAST('["棉","亚麻","羊毛","针织","牛仔","真丝","皮革","聚酯纤维","混纺","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'wardrobe.item_options'
  AND ` + "`key`" + ` = 'materials'`,
			`UPDATE system_configs
SET value = CAST('["春夏","春秋","秋冬","夏季","冬季","四季"]' AS JSON)
WHERE ` + "`group`" + ` = 'wardrobe.item_options'
  AND ` + "`key`" + ` = 'seasons'`,
			`UPDATE system_configs
SET value = CAST('["修身","合身","微宽松","宽松","直筒","A 字","短款","长款","高腰","其他"]' AS JSON)
WHERE ` + "`group`" + ` = 'wardrobe.item_options'
  AND ` + "`key`" + ` = 'silhouettes'`,
		},
	},
}
