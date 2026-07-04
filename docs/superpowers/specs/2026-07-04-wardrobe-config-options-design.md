# 衣橱枚举配置下拉设计

## 背景

新增衣服表单中的分类、材质、季节和廓形目前是自由输入。图片识别回填后，如果这些字段没有枚举约束，用户和 AI 都可能写入不稳定的表达，影响后续搭配建议和统计筛选。

`system_configs` 表已经存在，适合作为第一版的系统配置来源。本轮复用该表保存衣橱字段枚举。

## 目标

- 分类、材质、季节、廓形改为下拉选择。
- 枚举值保存到 `system_configs` 表。
- 小程序通过后端接口获取枚举配置。
- 后端保存衣服时强校验非空枚举值。
- 图片识别回填只能回填配置中存在的枚举值。

## 非目标

- 不改 `wardrobe_items` 主表字段类型。
- 不做后台配置管理页面。
- 不把所有衣服字段做成动态表单。
- 不影响颜色、名称、场景和备注的自由输入。

## 配置设计

使用 `system_configs`：

- `group`: `wardrobe.item_options`
- `key`: `categories`、`materials`、`seasons`、`silhouettes`
- `value_type`: `json`
- `value`: JSON 数组，每项包含 `label` 和 `value`
- `status`: `active`

示例：

```json
[
  { "label": "上装", "value": "top" },
  { "label": "下装", "value": "bottom" }
]
```

新增 migration 写入默认枚举，并使用 `ON DUPLICATE KEY UPDATE` 保持可重复执行。

## 后端设计

新增接口：

`GET /api/user/wardrobe/options`

响应：

```json
{
  "categories": [{ "label": "上装", "value": "top" }],
  "materials": [{ "label": "棉", "value": "cotton" }],
  "seasons": [{ "label": "春秋", "value": "spring_autumn" }],
  "silhouettes": [{ "label": "微宽松", "value": "slightly_relaxed" }]
}
```

保存强校验：

- `category` 必填，且必须在 `categories` 中。
- `material`、`season`、`silhouette` 可空；非空时必须在对应配置中。
- 配置读取失败时后端使用内置默认枚举兜底，避免配置缺失导致新增衣服完全不可用。

## 前端设计

小程序新增 API：

- `getWardrobeOptions()`

衣橱页新增：

- `wardrobeOptions`
- `materialOptions`
- `seasonOptions`
- `silhouetteOptions`

表单使用 `picker`：

- 分类 picker 必选，默认 `top` 或当前筛选分类。
- 材质、季节、廓形 picker 可选择，也可保持空。
- 展示用 label，保存用 value。

图片识别回填：

- 对 `category/material/season/silhouette` 先尝试匹配 `value`。
- 如果未匹配，再尝试匹配 `label`。
- 匹配成功才回填 value；匹配失败不回填该字段。
- 仍遵守“只填空字段”规则。

## 验证

- migration 测试覆盖默认配置 SQL。
- 后端 service/routes 测试覆盖配置读取、保存强校验、无效枚举错误。
- 小程序 API 验证覆盖 `getWardrobeOptions`。
- 小程序集成验证覆盖 picker 结构、选项加载、选择事件、识别值按枚举回填。
