# Onboarding 到初版报告闭环设计

日期：2026-06-22

## 背景

项目骨架已经完成，当前仓库包含微信小程序、用户 Web、管理端、Go 双服务入口、数据库初始迁移和少量占位业务模块。第一轮业务实现选择围绕 MVP 主闭环中的前半段展开：

```text
开发登录
→ 混合式 onboarding
→ 用户画像与偏好沉淀
→ 候选形象路线
→ 初版行动型报告
```

本设计只覆盖「Onboarding 到初版报告」这一条正式但很薄的端到端业务线。今日建议、建议反馈、完整衣橱管理、真实微信登录、真实 OSS 上传、管理端风格库维护和完整对话式 Agent SSE 不进入本轮范围。

## 目标

本轮目标是打通一个可独立验收的业务闭环：

1. 用户通过开发态登录进入系统。
2. 小程序按步骤保存 onboarding 草稿，每一步支持结构化字段和一句自由补充。
3. 用户提交 onboarding 后，服务端沉淀基础画像、明确事实、偏好禁忌、照片引用和核心衣橱。
4. 服务端通过可替换的报告生成器生成 1-3 条候选形象路线和初版行动型报告。
5. 前端可以查询生成任务状态、最新报告和指定报告。
6. 用户可以对候选形象路线做轻反馈，改变路线状态并记录事件。

## 非目标

本轮不做：

- 真实微信 `code2session` 登录。
- 完整 session 权限体系和多端登录管理。
- 七牛云上传 token、回调和真实对象存储清理。
- 完整衣橱库存管理。
- 今日建议生成和建议反馈闭环。
- 管理端明星风格库维护。
- 自动抓取明星图片。
- 纯对话式 onboarding 或 Agent SSE onboarding。
- 电商 SKU 推荐和商品链接。

## 用户体验形态

Onboarding 采用混合式：

```text
分步收集关键材料 + 每步一句自由补充
```

小程序继续沿用当前 6 个步骤方向：

1. 形象照片。
2. 基础特征。
3. 核心衣橱。
4. 风格目标与禁忌。
5. 参考风格。
6. 生成报告。

每一步都允许用户用一句话补充结构化选项无法表达的信息，例如「平时头发会更塌一点」「喜欢松弛感，不是想模仿长相」。后端保存原始草稿，并在提交时区分用户明确事实、偏好禁忌和 AI 推断。

## 模块边界

### account

提供最小开发登录能力。第一轮创建或返回测试用户，并颁发开发态 token。token 对应的会话信息存储在 Redis 中，MySQL 只保存用户账号，不保存登录 session。后续接入微信登录时替换登录入口和 session 颁发，不改变 onboarding 主流程。

### onboarding

本轮业务编排入口。负责读取草稿、增量保存草稿、提交草稿、校验最小信息，并调用 profile、asset、wardrobe、image_route、report 和 job 的 service 能力完成闭环。

草稿需要独立持久化。本轮新增 `onboarding_drafts` 表，保存当前用户的草稿 JSON、草稿状态、版本号和内容哈希。它不替代 `profiles`、`profile_facts` 或 `profile_prefs`；只有用户提交后，草稿才被沉淀为正式业务数据。

### profile

沉淀用户基础画像、明确事实、偏好和禁忌。本轮只实现 onboarding 提交需要的 service/repo 能力，不做完整画像 CRUD。

### asset

登记照片和参考图资产引用。第一轮不生成真实 OSS 上传 token，只保存客户端传入的 `asset_public_id`、类型、来源和说明，为后续七牛云接入保留边界。

### wardrobe

保存 onboarding 中出现的核心衣橱单品。第一轮只维护高频核心单品，不做完整库存、批量整理或复杂图片识别。

### image_route

保存 1-3 条候选形象路线。路线初始状态为 `candidate`，用户轻反馈后可进入 `active`、`refinement` 或 `archived`。

### report

保存初版行动型报告。报告正文使用 JSON 快照，便于快速演进报告结构，同时保留报告状态和版本。

### job

记录初版报告生成任务。默认 mock/rule 生成时可以同步完成，但仍然落 job，前端通过统一接口查询任务状态。

### generator

提供 `ReportGenerator` 抽象。本地默认使用规则生成器，配置打开后可以切换到真实 LLM 生成器。两种实现共用同一份输入输出 schema。

## 用户侧接口

### 开发登录

```text
POST /api/user/dev-login
```

用途：创建或返回一个测试用户，返回 `user_public_id` 和开发态 token。

开发态 token 写入 Redis，例如：

```text
hestia:user-session:{token} -> user_id、user_public_id、surface、created_at
```

第一轮请求通过 `Authorization: Bearer <token>` 鉴权。服务端中间件从 Redis 读取 session，并把 `user_id`、`user_public_id` 放入请求上下文。Redis 中查不到 token 时返回未登录错误。

示例入参：

```json
{
  "nickname": "测试用户",
  "dev_key": "ming-local"
}
```

示例响应：

```json
{
  "user_public_id": "usr_xxx",
  "token": "dev_xxx",
  "onboarding_status": "not_started"
}
```

### Onboarding 草稿

```text
GET /api/user/onboarding
PUT /api/user/onboarding
POST /api/user/onboarding/submit
```

`GET` 读取当前用户草稿和 onboarding 状态。

`PUT` 按步骤增量保存草稿。每次请求带 `step` 和完整或局部 `data`，后端合并到当前草稿快照。

`submit` 校验草稿并生成报告。若用户已有成功报告且草稿未变化，默认返回最新报告；若草稿更新后再次提交，则创建新的报告版本。

### Job 查询

```text
GET /api/user/jobs/:public_id
```

返回任务类型、状态、错误码、错误说明、关联报告 ID 和更新时间。

### 报告查询

```text
GET /api/user/reports/latest
GET /api/user/reports/:public_id
```

返回报告状态、报告正文、候选路线摘要、行动项、缺口单品建议、避雷项和参考风格逻辑。

### 路线反馈

```text
POST /api/user/image-routes/:public_id/feedback
```

支持动作：

- `like`：路线进入 `active`。
- `dislike`：路线进入 `archived`。
- `adjust`：路线进入 `refinement`，记录用户希望调整的原因。

反馈写入 `image_route_events`，用于后续推荐权重和记忆更新。

## Onboarding 草稿结构

草稿保存在服务端，结构示例：

```json
{
  "photos": [
    {
      "asset_public_id": "ast_selfie_001",
      "asset_type": "selfie",
      "note": "这是最近的自拍，平时头发会更塌一点"
    }
  ],
  "basic": {
    "gender": "female",
    "height_cm": 165,
    "occupation_scene": "通勤、偶尔见客户",
    "note": "想看起来更有精神，但不要太强势"
  },
  "wardrobe": {
    "items": [
      {
        "name": "米白短外套",
        "category": "outerwear",
        "color": "米白",
        "note": "常穿，但不知道怎么搭"
      }
    ],
    "note": "衣服大多是黑白灰"
  },
  "style_goal": {
    "goals": ["干净", "有气质", "亲和"],
    "avoidances": ["甜美", "紧身", "过度成熟"],
    "note": "喜欢松弛感，不想像网红风"
  },
  "reference_style": {
    "subjects": [
      {
        "name": "某明星或博主",
        "liked_points": ["松弛感", "浅色层次", "干净发型"],
        "note": "喜欢造型逻辑，不是想像她"
      }
    ],
    "uploaded_refs": [
      {
        "asset_public_id": "ast_ref_001",
        "note": "喜欢这张的配色"
      }
    ]
  }
}
```

照片字段优先使用已登记资产的 `asset_public_id`。本轮如果前端还没有真实上传或资产登记能力，也可以传 `client_ref` 和基础元信息；提交时由 asset service 创建一条本地模拟资产记录并返回正式 `asset_public_id`。

提交时后端将草稿沉淀到：

- `profiles`：基础画像摘要。
- `profile_facts`：用户明确事实。
- `profile_prefs`：偏好和禁忌。
- `profile_inferences`：AI 推断，必须带置信度和来源。
- `assets`：照片和参考图引用。
- `wardrobe_items`：核心衣橱单品。
- `image_routes`：候选形象路线。
- `reports`：初版报告 JSON。
- `jobs`：报告生成任务记录。

## 草稿存储

新增表 `onboarding_drafts`：

- `id`
- `public_id`
- `user_id`
- `status`：`draft`、`submitted`、`completed`、`failed`
- `current_step`
- `draft_data`：完整草稿 JSON
- `content_hash`：用于判断重复提交
- `version`：草稿版本
- `submitted_at`
- `created_at`
- `updated_at`
- `deleted_at`

约束和索引：

- `uk_onboarding_drafts_public_id`
- `uk_onboarding_drafts_user_active`：应用层保证每个用户只有一个未删除草稿
- `idx_onboarding_drafts_user_status`

`PUT /api/user/onboarding` 每次成功保存后递增 `version` 并更新 `content_hash`。`POST /api/user/onboarding/submit` 使用 `content_hash` 判断是否需要创建新的报告版本。

## 报告生成双模式

服务端定义报告生成接口：

```go
type ReportGenerator interface {
    GenerateInitialReport(ctx context.Context, input InitialReportInput) (*InitialReportResult, error)
}
```

### RuleReportGenerator

默认启用。根据 onboarding 草稿生成稳定、可测试的规则化报告。它服务本地开发、小程序联调和测试，不依赖模型稳定性和外部成本。

### LLMReportGenerator

使用同一份输入输出 schema 调用真实 LLM。第一轮可以先实现配置开关和最小适配，prompt 细化在后续独立迭代中完成。

### 生成流程

```text
校验草稿
→ upsert profile/facts/prefs/assets/wardrobe
→ 创建 job: initial_report_generation
→ 调用 ReportGenerator
→ 写入 profile_inferences
→ 写入 image_routes
→ 写入 reports
→ 更新 users.onboarding_status = completed
→ 更新 job = succeeded 或 failed
→ 返回 job_public_id 和 report_public_id
```

报告生成结果包含：

- `summary`：中性、行动型摘要。
- `routes`：1-3 条候选形象路线。
- `hair_strategy`：发型方向。
- `makeup_strategy`：妆容或气色方向。
- `outfit_strategy`：穿搭策略。
- `avoidances`：不建议做什么。
- `wardrobe_combinations`：现有衣橱可用组合。
- `wardrobe_gaps`：缺口单品建议，不含商品链接。
- `action_items`：今天或本周可执行动作。
- `reference_style_logic`：参考风格的可迁移逻辑和不可迁移风险。
- `privacy_note`：照片与档案可删除提示。

## 数据与隐私约束

- 自拍、身材、衣橱、审美偏好和反馈都按敏感个人数据处理。
- 用户会话信息只存 Redis，MySQL 不存登录 session。
- 用户明确事实和 AI 推断必须分表或分类型保存，不能混同。
- AI 推断必须带置信度、来源和可修正状态。
- 报告表达避免羞辱式或制造焦虑的措辞。
- 明星或参考图只表达可迁移造型逻辑，不表达「你像某明星」。
- 缺口单品建议只给品类和选择逻辑，不给具体商品链接。
- 本轮接口设计要为后续删除照片、档案和反馈数据保留入口和关联字段。

## 错误处理

- 草稿缺少最小信息时，`submit` 返回结构化校验错误，例如缺少基础场景或风格目标。
- generator 失败时，job 标记为 `failed`，记录错误码和简短原因。
- 来自用户明确提交的 profile、facts、prefs 可以保留；未成功生成的 report 和 image_routes 不返回给前端。
- 生成成功但报告查询失败时，前端可通过 job 关联的 `report_public_id` 重试查询。
- 重复提交时按草稿版本判断：草稿未变化返回最新成功报告，草稿变化创建新报告版本。

## 测试与验收

服务端测试覆盖：

- `POST /api/user/dev-login` 创建或返回测试用户。
- `GET/PUT /api/user/onboarding` 保存和读取混合式草稿。
- `POST /api/user/onboarding/submit` happy path。
- 草稿缺少最小信息时的校验错误。
- 重复提交时返回最新报告或创建新版本。
- `ReportGenerator` 失败时 job 状态变为 `failed`。
- `GET /api/user/jobs/:public_id` 查询任务状态。
- `GET /api/user/reports/latest` 和 `GET /api/user/reports/:public_id` 查询报告。
- `POST /api/user/image-routes/:public_id/feedback` 更新路线状态并记录事件。

前端验收：

- 小程序报告页能请求 `reports/latest` 并展示真实报告数据。
- 报告页保留 mock fallback，便于本地后端未启动时预览。
- Onboarding 页面可以按后端接口逐步接入，不要求本轮一次性完成完整交互 UI。

## 实施顺序

1. 实现 `account` 最小开发登录和简单鉴权上下文。
2. 实现 `onboarding` 草稿保存和读取。
3. 实现 `profile`、`asset`、`wardrobe` 的本轮最小 repo/service 能力。
4. 实现 `ReportGenerator` 接口和 `RuleReportGenerator`。
5. 实现 onboarding submit 编排和事务。
6. 实现 `image_route`、`report`、`job` 查询接口。
7. 实现路线反馈接口。
8. 小程序报告页接入 `reports/latest`，保留 mock fallback。
9. 补齐服务端测试和小程序验证说明。

## 后续演进

本轮完成后，下一批业务可以从以下方向选择：

- 接入真实微信登录和 session。
- 接入七牛云上传 token、资产回调和隐私删除。
- 把 `LLMReportGenerator` 从最小适配升级为稳定 prompt 与结构化输出。
- 实现今日建议和建议反馈闭环。
- 实现管理端风格库维护，让参考风格样本成为路线生成输入。
