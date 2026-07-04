INSERT IGNORE INTO `system_configs`
  (`group`, `key`, `value`, `value_type`, `description`, `status`)
VALUES
  (
    'wardrobe.item_options',
    'categories',
    CAST('["上装","下装","外套","鞋","包","配饰","运动","家居","其他"]' AS JSON),
    'json',
    '衣橱单品分类枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'colors',
    CAST('["黑色","白色","米白","灰色","深蓝","浅蓝","棕色","卡其","红色","绿色","其他"]' AS JSON),
    'json',
    '衣橱单品颜色建议',
    'active'
  ),
  (
    'wardrobe.item_options',
    'materials',
    CAST('["棉","亚麻","羊毛","针织","牛仔","真丝","皮革","聚酯纤维","混纺","其他"]' AS JSON),
    'json',
    '衣橱单品材质枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'seasons',
    CAST('["春夏","春秋","秋冬","夏季","冬季","四季"]' AS JSON),
    'json',
    '衣橱单品季节枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'silhouettes',
    CAST('["修身","合身","微宽松","宽松","直筒","A 字","短款","长款","高腰","其他"]' AS JSON),
    'json',
    '衣橱单品廓形枚举',
    'active'
  )
