INSERT INTO `system_configs`
  (`group`, `key`, `value`, `value_type`, `description`, `status`)
VALUES
  (
    'wardrobe.item_options',
    'categories',
    CAST('[
      {"label":"上装","value":"top"},
      {"label":"下装","value":"bottom"},
      {"label":"外套","value":"outerwear"},
      {"label":"鞋","value":"shoes"},
      {"label":"包","value":"bag"},
      {"label":"配饰","value":"accessory"},
      {"label":"运动","value":"sport"},
      {"label":"家居","value":"home"},
      {"label":"其他","value":"other"}
    ]' AS JSON),
    'json',
    '衣橱单品分类枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'materials',
    CAST('[
      {"label":"棉","value":"cotton"},
      {"label":"亚麻","value":"linen"},
      {"label":"羊毛","value":"wool"},
      {"label":"针织","value":"knit"},
      {"label":"牛仔","value":"denim"},
      {"label":"真丝","value":"silk"},
      {"label":"皮革","value":"leather"},
      {"label":"聚酯纤维","value":"polyester"},
      {"label":"混纺","value":"blend"},
      {"label":"其他","value":"other"}
    ]' AS JSON),
    'json',
    '衣橱单品材质枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'seasons',
    CAST('[
      {"label":"春夏","value":"spring_summer"},
      {"label":"春秋","value":"spring_autumn"},
      {"label":"秋冬","value":"autumn_winter"},
      {"label":"夏季","value":"summer"},
      {"label":"冬季","value":"winter"},
      {"label":"四季","value":"all_season"}
    ]' AS JSON),
    'json',
    '衣橱单品季节枚举',
    'active'
  ),
  (
    'wardrobe.item_options',
    'silhouettes',
    CAST('[
      {"label":"修身","value":"fitted"},
      {"label":"合身","value":"regular"},
      {"label":"微宽松","value":"slightly_relaxed"},
      {"label":"宽松","value":"relaxed"},
      {"label":"直筒","value":"straight"},
      {"label":"A 字","value":"a_line"},
      {"label":"短款","value":"cropped"},
      {"label":"长款","value":"longline"},
      {"label":"高腰","value":"high_waist"},
      {"label":"其他","value":"other"}
    ]' AS JSON),
    'json',
    '衣橱单品廓形枚举',
    'active'
  )
ON DUPLICATE KEY UPDATE
  `value` = VALUES(`value`),
  `value_type` = VALUES(`value_type`),
  `description` = VALUES(`description`),
  `status` = VALUES(`status`);
