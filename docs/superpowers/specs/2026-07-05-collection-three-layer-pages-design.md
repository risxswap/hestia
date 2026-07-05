# 私藏与我的页三层页面架构设计

## 背景

当前小程序部分页面把数据量较多的表单直接放在概览页或弹层里：

- `我的` 页直接展示基础档案、偏好与禁忌、隐私操作。
- `衣服` 页已有独立详情页和编辑页，新增字段较少，适合保留列表页轻量新增弹层。
- `发型`、`妆容` 目前只有空列表页，没有详情、编辑和完整数据闭环。

项目仍处于开发阶段，不需要兼容旧入口或旧交互。本轮目标是一次性建立清晰的页面层级和数据模型，避免未来把复杂字段继续堆在概览页、弹层或 JSON 字段中。

## 目标

- 全局采用“概览/列表页、详情页、编辑页”三层页面结构。
- 详情页和编辑页严格分开：详情页只读，编辑页承载所有表单和状态修改。
- 概览/列表页不直接展示数据量较多的表单。
- 弹层只用于轻量新增、删除确认、候选项选择、轻量筛选等辅助操作。
- `我的` 页改成档案概览和入口页。
- `衣服`、`发型`、`妆容` 的新增都使用列表页轻量弹层；详情页仍必须只读。
- `发型`、`妆容` 一次性补齐列表、详情、轻量新增、编辑、删除闭环。
- `发型`、`妆容` 后端使用独立表，核心业务字段尽量使用显式列，不把长期演进数据堆进 JSON。

## 非目标

- 不做完整电商导购、SKU 推荐或商品链接。
- 不自动抓取公开明星图片。
- 不做相貌对比，不输出“你像某明星”作为核心结论。
- 不做减肥、医疗、植发、皮肤病、整形等高风险建议。
- 不为了兼容旧页面结构保留同页大表单；私藏新增弹层是基于字段少和操作轻的产品决策，不是兼容旧实现。
- 不把所有私藏类型合并进一个通用 `collection_items` 大表。

## 全局页面规则

### 概览/列表页

职责：

- 展示摘要、计数、状态、筛选和入口。
- 展示轻量卡片，卡片点击进入详情页。
- 新增按钮打开列表页轻量新增弹层。
- 不展示大表单；只允许在轻量新增弹层里承载必要上传和保存。

适用页面：

- `pages/profile/index`
- `pages/collection/collection`
- `pages/clothes/list`
- `pages/hair/list`
- `pages/makeup/list`

### 详情页

职责：

- 严格只读。
- 展示图片、结构化档案、AI 解析摘要、适用场景、建议逻辑、反馈摘要和数据来源。
- 提供“编辑”“删除”“返回”等操作入口。
- 不出现输入框、上传控件、开关、状态切换或保存按钮。

适用页面：

- `pages/clothes/detail`
- `pages/hair/detail`
- `pages/makeup/detail`

### 编辑页

职责：

- 承载完整字段填写、推荐状态、保存；已有识别能力的类型可以在这里承载识别和回填。
- 编辑页使用 `public_id` 读取原有详情后填充表单。
- 保存失败必须保留用户输入。
- 上传或替换图片后可触发识别和字段回填，但回填内容需要允许用户修改。

适用页面：

- `pages/clothes/edit`
- `pages/hair/edit`
- `pages/makeup/edit`
- `pages/profile/edit`
- `pages/preferences/edit`

### 独立管理页

职责：

- 承载不是详情页、也不是表单编辑页的独立管理能力。
- 页面可以展示列表、分组、说明和轻量纠错操作。

适用页面：

- `pages/memory/index`
- `pages/privacy/index`

### 弹层允许范围

允许：

- 私藏新增这种字段少、无复杂解释信息的轻量新增弹层。
- 删除确认。
- 候选项选择，例如分类、颜色、季节。
- 轻量筛选、排序。
- 简短说明或不可逆操作确认。

不允许：

- 主表单。
- 图片上传，私藏轻量新增弹层除外。
- 数据量较多的详情展示。
- 需要保存的多字段编辑。

## 页面矩阵

| 对象 | 列表/概览页 | 详情页 | 新增方式 | 编辑/管理页 |
|---|---|---|---|---|
| 我的 | `pages/profile/index` | 不做传统详情，使用只读摘要卡 | 不适用 | `pages/profile/edit`、`pages/preferences/edit`、`pages/memory/index`、`pages/privacy/index` |
| 衣服 | `pages/clothes/list` | `pages/clothes/detail` | 列表页轻量弹层 | `pages/clothes/edit?public_id=...` |
| 发型 | `pages/hair/list` | `pages/hair/detail` | 列表页轻量弹层 | `pages/hair/edit?public_id=...` |
| 妆容 | `pages/makeup/list` | `pages/makeup/detail` | 列表页轻量弹层 | `pages/makeup/edit?public_id=...` |

## 小程序页面设计

### 我的页

`pages/profile/index` 改为概览页。

保留：

- 顶部身份区。
- 长期记忆摘要入口。
- 快捷入口卡片。

移除：

- 基础档案同页表单。
- 偏好与禁忌同页表单。
- 隐私操作同页面板。

入口跳转：

- 基础档案：`/pages/profile/edit`
- 偏好与禁忌：`/pages/preferences/edit`
- 记忆：`/pages/memory/index`
- 报告与路线：`/pages/report/report`
- 隐私与数据：`/pages/privacy/index`
- 补充档案：`/pages/onboarding/onboarding`

### 基础档案编辑页

`pages/profile/edit`

字段：

- 昵称。
- 性别表达。
- 身高。
- 常见场景。
- 身形/比例备注。
- 肤色/妆发备注。
- 发型备注。

提交接口复用：

- `PATCH /api/user/profile`

### 偏好与禁忌编辑页

`pages/preferences/edit`

字段：

- 风格目标。
- 不想要的方向。
- 场景偏好。

提交接口复用：

- `PATCH /api/user/profile/preferences`

### 记忆页

`pages/memory/index`

首版能力：

- 展示长期记忆列表和摘要统计。
- 区分用户明确陈述的事实、偏好、禁忌和 AI 推断。
- 对 AI 推断展示不确定性和来源说明。
- 提供轻量纠错入口，例如标记不准、待确认、后续补充。
- 不在我的概览页直接展开完整记忆列表。

### 隐私与数据页

`pages/privacy/index`

首版能力：

- 清除本地登录。
- 服务端照片、档案、反馈删除入口可以先展示不可用状态，但页面结构预留。
- 危险操作必须二次确认，并说明影响范围。

### 衣服页

`pages/clothes/list`

调整：

- 保留筛选、图库、建议补齐。
- 保留列表页轻量新增弹层，因为衣服新增只需要上传图片并保存，复杂字段后续进入编辑页完善。
- 新增弹层不得扩展为完整衣服编辑表单；如果新增字段增长，应重新评估是否需要独立新增页。
- 卡片点击进入 `pages/clothes/detail?public_id=...`。

`pages/clothes/detail`

要求：

- 保持只读。
- 只保留编辑和删除入口。
- 不承载任何字段编辑。

`pages/clothes/edit`

调整：

- 仅承载已有衣服的编辑，使用 `public_id` 读取原有详情后填充表单。

### 发型页

新增页面：

- `pages/hair/detail`
- `pages/hair/edit`

`pages/hair/list`：

- 展示发型卡片列表。
- 空态引导新增。
- 新增按钮打开轻量新增弹层。
- 新增弹层只采集图片、名称和少量必要字段；完整字段进入编辑页完善。
- 卡片点击进入 `pages/hair/detail?public_id=...`。

`pages/hair/detail` 只读展示：

- 主图。
- 名称、长度、轮廓/层次、刘海、发色、打理时间。
- 适用场景。
- 适合原因。
- 不建议照做的点。
- 用户备注。
- 推荐状态。

`pages/hair/edit` 字段：

- 图片。
- 名称。
- 长度。
- 轮廓/层次。
- 刘海。
- 发色。
- 打理时间。
- 适用场景。
- 适合原因。
- 不建议照做的点。
- 用户备注。
- 推荐状态。

### 妆容页

新增页面：

- `pages/makeup/detail`
- `pages/makeup/edit`

`pages/makeup/list`：

- 展示妆容卡片列表。
- 空态引导新增。
- 新增按钮打开轻量新增弹层。
- 新增弹层只采集图片、名称和少量必要字段；完整字段进入编辑页完善。
- 卡片点击进入 `pages/makeup/detail?public_id=...`。

`pages/makeup/detail` 只读展示：

- 主图。
- 名称、妆容类型、妆面重点、色彩方向、妆感。
- 适用场景。
- 适合原因。
- 不建议照做的点。
- 用户备注。
- 推荐状态。

`pages/makeup/edit` 字段：

- 图片。
- 名称。
- 妆容类型。
- 妆面重点。
- 色彩方向。
- 妆感。
- 适用场景。
- 适合原因。
- 不建议照做的点。
- 用户备注。
- 推荐状态。

## 后端接口设计

### 衣服

接口：

- `GET /api/user/clothes/options`
- `GET /api/user/clothes/items`
- `POST /api/user/clothes/items`
- `POST /api/user/clothes/items/recognize`
- `PATCH /api/user/clothes/items/:public_id`
- `DELETE /api/user/clothes/items/:public_id`

补充：

- 新增 `GET /api/user/clothes/items/:public_id`，避免详情页从列表中过滤。

### 发型

新增接口：

- `GET /api/user/hair`
- `GET /api/user/hair/:public_id`
- `POST /api/user/hair`
- `PATCH /api/user/hair/:public_id`
- `DELETE /api/user/hair/:public_id`

### 妆容

新增接口：

- `GET /api/user/makeup`
- `GET /api/user/makeup/:public_id`
- `POST /api/user/makeup`
- `PATCH /api/user/makeup/:public_id`
- `DELETE /api/user/makeup/:public_id`

### 私藏聚合

`GET /api/user/collection` 继续作为聚合接口。

调整：

- 三类数量都来自真实领域表。
- `recent_items` 聚合衣服、发型、妆容最近更新项。
- `entry_path` 指向各自列表页或详情页。

## 数据库设计

### 设计原则

- `发型`、`妆容` 分别使用独立表。
- 核心业务字段使用显式列。
- 多值标签使用独立子表，不在主表里使用 JSON 数组。
- 图片关系使用独立关联表。
- 所有主表支持软删除。

### clothes

衣服是具体对象，衣服集合才是衣橱。数据表、接口和页面域统一使用 `clothes`，不使用 `wardrobe` 表达具体衣服。

建议字段沿用当前衣服字段：

- `id`
- `public_id`
- `user_id`
- `name`
- `category`
- `color`
- `silhouette`
- `material`
- `season`
- `user_notes`
- `is_core`
- `recommendation_status`
- `recognition_status`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

关联表：

- `clothes_assets`
  - `id`
  - `clothes_id`
  - `asset_id`
  - `is_primary`
  - `sort_order`
  - `created_at`

### hair

建议字段：

- `id`
- `public_id`
- `user_id`
- `name`
- `length`
- `shape`
- `bangs`
- `color`
- `care_time`
- `suitability_notes`
- `avoidance_notes`
- `user_notes`
- `recommendation_status`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

关联表：

- `hair_assets`
  - `id`
  - `hair_id`
  - `asset_id`
  - `is_primary`
  - `sort_order`
  - `created_at`

标签表：

- `hair_scene_tags`
  - `id`
  - `hair_id`
  - `tag`
  - `sort_order`
  - `created_at`

### makeup

建议字段：

- `id`
- `public_id`
- `user_id`
- `name`
- `makeup_type`
- `focus`
- `color_palette`
- `finish`
- `suitability_notes`
- `avoidance_notes`
- `user_notes`
- `recommendation_status`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

关联表：

- `makeup_assets`
  - `id`
  - `makeup_id`
  - `asset_id`
  - `is_primary`
  - `sort_order`
  - `created_at`

标签表：

- `makeup_scene_tags`
  - `id`
  - `makeup_id`
  - `tag`
  - `sort_order`
  - `created_at`

### JSON 使用边界

原则上不在本轮新增表中使用 JSON 字段。

不允许：

- 用 `metadata_json` 承载核心业务字段。
- 用 `ai_attrs` 承载未设计清楚但会参与页面展示和推荐的长期字段。
- 把不同类型对象混入一个通用 JSON 字段来逃避建模。
- 把标签、适用场景、风格关键词放进主表 JSON 数组。

若后续需要 AI 解析结果，应优先设计独立字段或独立来源表，并标明事实、推断、置信度和来源。

## 前端 API 客户端

`miniapp/utils/api.js` 新增或扩展：

- `getHairItems`
- `getHairItem`
- `createHairItem`
- `updateHairItem`
- `deleteHairItem`
- `getMakeupItems`
- `getMakeupItem`
- `createMakeupItem`
- `updateMakeupItem`
- `deleteMakeupItem`

衣服补充：

- `getClothesItem`

## 错误与空态

- 列表读取失败：展示错误卡片和重试按钮。
- 详情读取失败：展示错误卡片，不展示空详情。
- 保存失败：留在编辑页，保留用户输入。
- 删除失败：保留在详情页，提示失败原因。
- 图片上传失败：保留已选字段，允许重试上传。
- 无图片对象：详情页用中性占位，不阻塞保存。

## 隐私与安全

- 自拍、衣服、发型、妆容、偏好和反馈都视为敏感个人数据。
- 图片资产必须归属当前用户，跨用户资产不可关联。
- 删除对象默认软删除；服务端删除资产能力后续独立设计。
- 危险操作必须二次确认。
- 文案避免羞辱式、焦虑式表达。
- 发型和妆容建议不输出医疗诊断或需要专业资质的判断。

## 测试与验证

服务端：

- 路由测试覆盖三类 CRUD。
- Repository 测试覆盖创建、列表、详情、更新、软删除、资产关联。
- Collection 聚合测试覆盖三类计数和最近项。
- 资产归属校验测试覆盖跨用户资产不可关联。

小程序：

- API 客户端验证脚本覆盖新增接口。
- 页面脚本单元验证覆盖列表标准化、详情跳转、轻量新增 payload、编辑 payload 生成。
- 现有 `verify-miniapp-api-integration.js` 更新路由和接口检查。
- 手动验证路径：
  - 我的页进入基础档案编辑并保存。
  - 我的页进入偏好编辑并保存。
  - 我的页进入记忆页并查看事实、偏好、禁忌和推断分组。
  - 衣服、发型、妆容列表通过轻量弹层新增。
  - 衣服详情只读，编辑按钮进入编辑页。
  - 发型、妆容分别完成轻量新增、详情、编辑、删除。

## 实施顺序建议

1. 数据库 migration：新增发型、妆容主表、资产关联表和标签表。
2. 后端领域：为 hair、makeup 补齐 model、repo、service、handler、routes、tests。
3. Collection 聚合：接入三类真实数量和最近项。
4. 小程序 API：补齐 CRUD 客户端方法。
5. 我的页拆分：新增 `pages/profile/edit`、`pages/preferences/edit`、`pages/memory/index`、`pages/privacy/index`，移除同页表单和完整记忆列表。
6. 私藏新增收口：衣服、发型、妆容都使用列表页轻量新增弹层，确保详情页只读、编辑页只处理已有对象。
7. 发型、妆容页面闭环：列表、详情、编辑。
8. 验证脚本和手动路径验证。
