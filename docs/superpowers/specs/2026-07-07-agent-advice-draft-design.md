# 智能体对话创建建议草稿设计

## 背景

Hestia 的核心交互应从“填表生成一次性建议”逐步转向“通过智能体持续对话创建和精修形象建议”。用户可以说出场景、限制、偏好和临时想法，智能体用 ReAct 模式自主决定读取哪些上下文、是否创建草稿、是否基于当前草稿继续精修。

第一版建议使用 Eino 的 ReAct Agent 和受控 tools。大模型负责理解意图、规划步骤和生成建议内容；后端负责权限、状态机、结构化落库和隐私边界。

## 目标

- 使用聊天作为穿搭、发型、妆容建议的创建入口。
- 使用 Eino `flow/agent/react` 实现 ReAct Agent。
- 将用户档案、记忆、衣物查询和草稿写入封装为 Agent tools。
- 支持用户通过后续对话精细化控制当前草稿。
- 第一版不新增独立 chat session 表；当前草稿范围定义为当前用户在 advisor 入口下最近的 `draft` 草稿。
- 草稿先落库，用户点击按钮确认后才创建正式建议。
- 草稿使用受控 JSON 承载穿搭、发型、妆容正文，并通过 schema 版本和 tool 校验避免变成垃圾堆。
- 穿搭、发型、妆容三类内容都结构化进入草稿和正式建议。

## 非目标

- 不做完整电商导购或 SKU 推荐。
- 不通过自然语言直接确认保存正式建议。
- 不让 LLM 直接访问数据库或拼 SQL。
- 不实现多个草稿并列对比。
- 不做完整草稿版本 UI、回滚 UI 或 diff UI。
- 不输出医疗诊断、皮肤病、过敏判断、整形、植发等高风险建议。

## 总体架构

`server/internal/domain/agent` 作为聊天编排入口。每条用户消息先写入 `chat_msgs`，再构造带系统约束、历史消息和当前用户上下文的 Eino ReAct Agent 输入。

Agent 只能调用白名单 tools。读上下文 tools 可以查询档案、记忆和衣物；写草稿 tools 只能创建、更新或废弃当前用户自己的草稿。正式保存不暴露给 Agent，只通过前端按钮调用确认接口。

```text
miniapp pages/advisor
  -> POST /api/user/agent/chat
  -> 写 chat_msgs(user)
  -> 创建 chat_msgs(assistant,status=generating)
  -> Eino ReAct Agent
       -> get_profile_context
       -> get_memory_context
       -> get_wardrobe_context
       -> get_current_advice_draft
       -> create_advice_draft / update_advice_draft / discard_advice_draft
  -> 写 agent_run_steps
  -> 更新 chat_msgs(assistant,status=sent|failed)
  -> SSE 返回助手文案和草稿卡片

用户点击保存草稿
  -> POST /api/user/advice-drafts/:public_id/confirm
  -> 读取当前版本
  -> 创建 advices(status=ready)
  -> advice_drafts.status = confirmed
```

## 上下文管理

第一版不新增会话摘要表，也不让 LLM 直接读取全量用户数据。每轮 Agent 运行只装配完成当前任务所需的最小上下文，其余信息通过 tools 按需读取。

### 上下文装配

`POST /api/user/agent/chat` 每轮装配：

```text
system prompt
+ 最近 N 条 chat_msgs
+ 当前草稿轻量摘要
+ 当前用户消息
+ tool definitions
```

装配规则：

- `system prompt` 只包含角色、产品边界、隐私边界、tool 使用规则和保存限制，不塞业务数据。
- 最近聊天历史默认取最近 `8-12` 条 `chat_msgs`，按 token 预算裁剪。
- 聊天历史只保留用户可见文本和草稿卡片摘要，不重复注入完整草稿结构。
- 当前草稿默认只注入轻量摘要：`draft_public_id`、当前版本号、场景、穿搭/发型/妆容短摘要。
- 当前用户消息必须完整保留，不被摘要替代。
- tool definitions 每轮提供给 ReAct Agent，但 tool 返回内容必须短、相关、可解释。

### 按需读取

以下数据不默认进入 prompt：

- 完整用户档案。
- 全量长期记忆。
- 全量衣物列表。
- 完整草稿版本内容。
- `agent_run_steps` 决策步骤。

Agent 需要时通过 tools 读取：

- `get_profile_context`：读取与当前问题相关的明确档案、偏好、禁忌和可展示推断。
- `get_memory_context`：读取与当前场景相关的高置信或最近记忆。
- `get_wardrobe_context`：读取与当前场景、季节、品类或用户要求相关的可推荐衣物。
- `get_current_advice_draft`：读取当前草稿的完整结构化内容。

`agent_run_steps` 只用于审计、排错和重试恢复，不作为常规上下文喂回 LLM。

### 裁剪优先级

超过上下文预算时，按以下优先级保留：

1. 系统边界和安全规则。
2. 当前用户消息。
3. 当前草稿轻量摘要。
4. 最近用户明确约束，例如“不要露腿”“妆淡一点”。
5. 最近助手草稿摘要。
6. 普通寒暄和低信息密度回复。

第一版不建立 `chat_context_summaries`。当聊天历史明显变长、成本不可控或用户频繁跨多轮引用旧约束时，再新增会话摘要能力。

## Agent 决策边界

聊天意图由 LLM 在 ReAct 循环里判断，不单独实现一套后端规则引擎。

Agent 根据系统提示、用户消息、历史消息和当前草稿状态自主选择：

- 普通回答：不调用写入 tool。
- 需要生成形象方案：调用 `create_advice_draft`。
- 用户继续精修草稿：调用 `get_current_advice_draft` 和 `update_advice_draft`。
- 用户想重来：调用 `discard_advice_draft`，或在不确定时先追问。
- 用户想保存：提示用户点击草稿卡片上的保存按钮。

后端只提供硬约束：

- 没有当前草稿时，`update_advice_draft` 返回明确错误。
- 已确认或废弃的草稿不能更新。
- 不存在确认保存 tool，LLM 无法通过自然语言保存正式建议。
- 写入 tool 必须绑定当前用户和当前 `source_msg_id`。
- tool 参数必须结构化校验。
- 缺少必要场景信息时，Agent 可以追问，不强行生成。
- Agent 每轮运行由助手消息承载，可观察决策步骤必须记录到审计表。
- 不记录模型隐藏思维链全文，只记录模型输出的用户可见回复、tool 调用、tool 结果摘要和后端生成的决策摘要。

## 数据模型

草稿体系直接由用户消息触发，不再单独引入 `advice_requests`。用户原始输入、图片和触发来源由 `chat_msgs(user)` 承载，草稿通过 `source_msg_id` 回指触发消息。

```text
chat_msgs(user)
  -> chat_msgs(assistant)
       -> agent_run_steps(assistant_msg_id)
  -> advice_drafts(source_msg_id)
  -> advice_draft_versions(draft_id)
```

### `chat_msgs`

聊天消息表同时承载用户可见消息和一次 Agent 执行的头记录。

关键字段和扩展字段：

| 字段 | 备注 |
| --- | --- |
| `id` | 内部自增 ID。 |
| `public_id` | 对外暴露 ID，用于前端和接口引用。 |
| `user_id` | 消息所属用户。 |
| `source_msg_id` | 新增可空字段；仅助手消息使用，指向触发本轮 Agent 的用户消息。 |
| `role` | 消息角色：`user` / `assistant`。 |
| `msg_type` | 消息类型：`text` / `draft_card` / `error`。 |
| `content_text` | 用户可见文本；草稿卡片消息中存助手引导文案。 |
| `content_json` | 仅保留已有兼容字段；第一版不把草稿主数据塞入这里。 |
| `asset_refs` | 消息关联的用户上传图片或素材引用。 |
| `related_type` | 关联业务对象类型，例如 `advice_draft`。 |
| `related_id` | 关联业务对象内部 ID。 |
| `related_public_id` | 关联业务对象对外 ID，例如草稿 public id。 |
| `job_id` | 关联异步任务 ID；Agent chat 第一版通常为空。 |
| `status` | 消息状态：`sent` / `generating` / `failed` 等。 |

约束：

- 用户消息写 `role=user,status=sent`。
- Agent 收到用户消息后先创建助手占位消息：`role=assistant,status=generating,source_msg_id=用户消息 id`。
- Agent 完成后更新同一条助手消息：`status=sent`，并写入 `content_text`、`msg_type` 和业务关联。
- Agent 失败后更新同一条助手消息：`status=failed,msg_type=error`，并写入可展示错误。
- `source_msg_id` 为可空字段；用户消息为空，助手消息指向触发本轮 Agent 的用户消息。
- `related_type` 和 `related_public_id` 用于关联本次助手消息生成或修改的草稿，例如 `related_type=advice_draft`。
- `chat_msgs` 不存模型、prompt、耗时和步骤审计信息；这些写入 `agent_run_steps`。

### `agent_run_steps`

Agent 可观察决策步骤表。记录 ReAct 循环里的模型回合、tool 调用、tool 结果和最终输出。

字段：

| 字段 | 备注 |
| --- | --- |
| `id` | 内部自增 ID。 |
| `public_id` | 对外暴露 ID，便于排障定位。 |
| `user_id` | 步骤所属用户，用于权限隔离和审计查询。 |
| `source_msg_id` | 本轮用户消息 ID。 |
| `assistant_msg_id` | 本轮 Agent 执行对应的助手占位消息 ID。 |
| `step_no` | 步骤序号，在同一 `assistant_msg_id` 下递增。 |
| `step_type` | 步骤类型：`model_decision` / `tool_call` / `tool_result` / `final_response` / `error`。 |
| `status` | 步骤状态：`running` / `succeeded` / `failed` / `skipped`。 |
| `usage_key` | LLM 使用场景，例如 `agent_chat`；至少第一条模型步骤写入。 |
| `provider_code` | 模型供应商编码，例如 `qwen`、`openai`。 |
| `model_code` | 实际调用的模型编码。 |
| `prompt_version` | 本轮系统提示词版本，便于回溯效果。 |
| `max_step` | 本轮 ReAct 最大步数配置。 |
| `tool_name` | tool 调用或结果对应的工具名；非 tool 步骤为空。 |
| `tool_call_id` | 模型生成的 tool call 标识，用于关联调用和结果。 |
| `decision_label` | 后端压缩后的决策标签，例如 `answer`、`create_draft`、`update_draft`、`read_context`。 |
| `input_summary` | 输入摘要；不存完整 prompt、图片内容或大段参数。 |
| `output_summary` | 输出摘要；不存隐藏思维链或未经压缩的模型中间文本。 |
| `related_type` | 关联业务对象类型，例如 `advice_draft`、`advice_draft_version`、`clothes_item`。 |
| `related_id` | 关联业务对象内部 ID。 |
| `related_public_id` | 关联业务对象对外 ID。 |
| `started_at` | 步骤开始时间。 |
| `finished_at` | 步骤结束时间。 |
| `duration_ms` | 步骤耗时，单位毫秒。 |
| `error_message` | 失败摘要，不写敏感原文或完整异常堆栈。 |
| `created_at` | 记录创建时间。 |

约束：

- `(assistant_msg_id, step_no)` 唯一。
- `source_msg_id` 指向本轮用户消息。
- `assistant_msg_id` 指向本轮 Agent 执行对应的助手占位消息。
- `usage_key`、`provider_code`、`model_code`、`prompt_version`、`max_step` 至少在第一条 `model_decision` 步骤写入。
- `tool_call` 和 `tool_result` 使用相同 `tool_call_id` 关联。
- `input_summary` 和 `output_summary` 使用短文本，不存完整大段 prompt、图片内容或任意 JSON。
- 写入型 tool 成功后必须写 `related_type` 和 `related_public_id`，方便追踪是哪一步创建或修改了草稿。
- 不记录模型隐藏思维链；如果模型输出了面向用户或面向 tool 的可见理由，只能压缩成 `decision_label` 和 `output_summary`。

### `advice_drafts`

草稿容器表。

字段：

| 字段 | 备注 |
| --- | --- |
| `id` | 内部自增 ID。 |
| `public_id` | 草稿对外 ID，用于卡片、确认和废弃接口。 |
| `user_id` | 草稿所属用户。 |
| `source_msg_id` | 创建该草稿的用户消息 ID。 |
| `status` | 草稿状态：`draft` / `confirmed` / `discarded`。 |
| `scene_key` | 标准化场景 key，例如 `work_meeting`、`date`；可为空。 |
| `scene_label` | 用户可见场景文案，例如“明天见客户”。 |
| `target_date` | 建议目标日期，例如今日、明日或用户指定日期。 |
| `occasion` | 具体场合描述，例如“客户拜访”“朋友聚会”。 |
| `weather_text` | 用户提供或系统获取的天气文本摘要。 |
| `mood_text` | 用户想呈现的状态或情绪，例如“轻松但专业”。 |
| `style_goal` | 本次方案目标，例如“更利落”“显得年轻一点”。 |
| `avoid_goal` | 本次明确避雷，例如“不要露腿”“不要太正式”。 |
| `current_version_no` | 当前草稿版本号，指向最新版。 |
| `content_schema_version` | 三段建议 JSON 的 schema 版本，例如 `v1`。 |
| `outfit_advice` | 当前版穿搭建议 JSON，结构由 schema 约束。 |
| `hair_advice` | 当前版发型建议 JSON，结构由 schema 约束。 |
| `makeup_advice` | 当前版妆容建议 JSON，结构由 schema 约束。 |
| `confirmed_advice_id` | 确认保存后关联的正式建议 ID。 |
| `created_at` | 创建时间。 |
| `updated_at` | 更新时间。 |
| `deleted_at` | 软删除时间。 |

约束：

- 当前草稿通过 `user_id + status=draft + updated_at` 查询。
- 第一版每个用户在 advisor 入口当前只操作最近一个 `draft` 草稿。
- `outfit_advice`、`hair_advice`、`makeup_advice` 只承载建议正文，不存用户档案、聊天历史或 tool 原始结果。
- 三段 JSON 必须由后端 tool 参数校验后写入，不能透传模型任意 JSON。
- `confirmed_advice_id` 在确认保存后写入。

### `advice_draft_versions`

草稿版本表，每轮创建或精修都新增一条。

字段：

| 字段 | 备注 |
| --- | --- |
| `id` | 内部自增 ID。 |
| `public_id` | 草稿版本对外 ID。 |
| `draft_id` | 所属草稿 ID。 |
| `user_id` | 所属用户。 |
| `version_no` | 版本号，从 1 开始递增。 |
| `source_msg_id` | 触发本次版本创建或精修的用户消息 ID。 |
| `user_intent` | 用户本轮修改意图摘要，例如“鞋子换舒服点”。 |
| `revision_summary` | 本轮版本变化摘要，供卡片和审计展示。 |
| `changed_outfit` | 本轮是否修改穿搭 JSON 分段。 |
| `changed_hair` | 本轮是否修改发型 JSON 分段。 |
| `changed_makeup` | 本轮是否修改妆容 JSON 分段。 |
| `content_schema_version` | 本版本三段建议 JSON 的 schema 版本。 |
| `outfit_advice` | 本版本穿搭建议 JSON 快照。 |
| `hair_advice` | 本版本发型建议 JSON 快照。 |
| `makeup_advice` | 本版本妆容建议 JSON 快照。 |
| `created_at` | 版本创建时间。 |

约束：

- `(draft_id, version_no)` 唯一。
- 当前版本由 `advice_drafts.current_version_no` 指向。
- 每个版本必须保存穿搭、发型、妆容三段 JSON 快照。未修改 JSON 分段从上一版复制。

### 三段建议 JSON schema

`outfit_advice`、`hair_advice`、`makeup_advice` 使用 JSON 是为了保留内容表达弹性，不代表可以存任意数据。第一版由 tool 入参结构体和后端校验约束字段，后续 schema 稳定后再考虑拆表。

共同约束：

- 每段 JSON 必须包含 `title`、`summary`、`why_text`、`avoid_text`、`alternative_text`。
- 每段 JSON 只存建议正文，不存用户档案、聊天历史、tool 原始结果或模型中间输出。
- 每段 JSON 的字段由 `content_schema_version` 解释；schema 变更必须升级版本。
- 写入前做白名单字段校验，未知字段拒绝或丢弃并记录错误摘要。
- 返回前端前由后端转换成稳定卡片 payload，前端不直接依赖模型原始字段。

穿搭建议字段：

- `title`：穿搭标题。
- `summary`：整体摘要。
- `items`：单品建议数组，每项包含 `role`、`text`、`source_type`、`source_public_id`、`reason_text`。
- `color_strategy`：色彩策略。
- `silhouette_strategy`：廓形策略。
- `material_strategy`：材质策略。
- `why_text`：为什么适合。
- `avoid_text`：不建议做什么。
- `alternative_text`：替代方案。

发型建议字段：

- `title`：发型标题。
- `summary`：整体摘要。
- `length_direction`：长度或扎发方向。
- `shape_direction`：轮廓方向。
- `bangs_direction`：刘海或额前处理方向。
- `styling_steps`：可执行打理步骤。
- `hold_level`：定型强度。
- `why_text`：为什么适合。
- `avoid_text`：不建议做什么。
- `alternative_text`：替代方案。

妆容建议字段：

- `title`：妆容标题。
- `summary`：整体摘要。
- `base_direction`：底妆方向，只描述妆效，不做肤质诊断。
- `brow_direction`：眉形和眉色方向。
- `eye_direction`：眼妆方向。
- `lip_direction`：唇妆方向。
- `cheek_direction`：腮红或修容方向，避免医疗或骨相诊断表达。
- `finish_level`：整体妆感强度。
- `step_text`：可执行步骤摘要。
- `why_text`：为什么适合。
- `avoid_text`：不建议做什么。
- `alternative_text`：替代方案。
- `safety_note`：安全边界提示，例如敏感不适时停止使用并咨询专业人士。

妆容边界：只做场景表达、色彩、浓淡、步骤和避雷；不做肤质诊断、过敏判断、治疗建议或医疗结论。

### 草稿衣物引用

草稿阶段不建立独立衣物引用表。衣物引用只是临时方案内容，先放在 `outfit_advice.items` JSON 中；只有用户确认草稿成为正式建议时，后端才校验并写入正式建议的结构化引用。

`outfit_advice.items` 每项建议字段：

- `role`：衣物角色，例如 `top`、`bottom`、`outer`、`shoe`、`bag`、`accessory`。
- `text`：前端展示文本，例如“米白衬衫”或“低跟乐福鞋”。
- `source_type`：来源类型，`wardrobe_item` 表示用户已有衣物，`gap_item` 表示缺口单品或泛化建议。
- `source_public_id`：当 `source_type=wardrobe_item` 时填写衣物 public id；缺口单品为空。
- `reason_text`：选择该衣物或单品方向的理由。

约束：

- 草稿阶段不把衣物引用计入长期统计。
- 草稿阶段不因衣物引用失效而阻断用户继续精修；前端可按文本展示。
- 确认保存时，后端重新校验 `source_type=wardrobe_item` 的 `source_public_id` 是否属于当前用户、未删除、未暂停推荐。
- 校验失败的衣物引用不写入正式建议引用，确认接口返回可展示提示或让用户回到草稿继续调整。
- `source_type=gap_item` 不生成正式衣物引用，只进入正式建议的缺口单品建议。

## Agent Tools

### 读上下文 tools

`get_profile_context`

- 读取明确档案、偏好、禁忌和可展示的 AI 推断。
- 返回时区分用户明确陈述和 AI 推断。
- AI 推断带置信度或不确定表达。

`get_memory_context`

- 读取最近或高置信记忆。
- 区分事实、偏好、禁忌、推断、待确认。
- 已删除或不可见记忆不得返回。

`get_wardrobe_context`

- 读取可推荐衣物。
- 支持按场景、季节、品类、常穿状态和暂停推荐状态过滤。
- 返回衣物 public id、名称、品类、颜色、廓形、材质、场景标签、用户备注。

`get_current_advice_draft`

- 读取当前用户最近 `status=draft` 草稿和当前版本。
- 返回场景字段、版本号和三段建议 JSON。

### 写草稿 tools

`create_advice_draft`

- 创建 `advice_drafts(status=draft)`。
- 创建 `advice_draft_versions(version_no=1)`。
- 写入 `outfit_advice`、`hair_advice`、`makeup_advice` 三段受控 JSON。
- 返回草稿 public id、版本号和可渲染卡片数据。

`update_advice_draft`

- 读取当前用户自己的 `status=draft` 草稿。
- 基于当前版本创建 `version_no + 1`。
- 修改指定 JSON 分段，未修改 JSON 分段从上一版复制。
- 更新 `advice_drafts.current_version_no`。
- 同步更新 `advice_drafts` 当前三段 JSON。
- 返回新版草稿卡片数据。

`discard_advice_draft`

- 仅废弃当前用户自己的 `status=draft` 草稿。
- 将 `advice_drafts.status` 改为 `discarded`。

### 决策步骤记录

Agent Runner 负责记录运行和步骤，不把记录职责交给 LLM。

记录流程：

1. 收到用户消息并写入 `chat_msgs(role=user,status=sent)`。
2. 创建本轮 Agent 对应的助手占位消息：`chat_msgs(role=assistant,status=generating,source_msg_id=用户消息 id)`。
3. 每次进入模型前写 `agent_run_steps(step_type=model_decision,status=running)`，并关联 `source_msg_id` 和 `assistant_msg_id`。
4. 第一条 `model_decision` 步骤写入 `usage_key`、`provider_code`、`model_code`、`prompt_version` 和 `max_step`。
5. 模型返回 tool call 后，将该步骤更新为 `succeeded`，写入 `decision_label` 和 `output_summary`。
6. 调用 tool 前写 `agent_run_steps(step_type=tool_call,status=running)`。
7. tool 返回后写 `agent_run_steps(step_type=tool_result,status=succeeded|failed)`。
8. 如果 tool 创建或修改草稿，步骤中写入 `related_type`、`related_id`、`related_public_id`。
9. 写入最终回复后，补一条 `final_response` 步骤。
10. 更新助手占位消息为 `status=sent` 或 `status=failed`，并写入 `content_text`、`msg_type` 和业务关联。

记录内容：

- 可以记录：调用了哪个 tool、输入摘要、输出摘要、关联草稿或版本、耗时、错误。
- 不记录：隐藏思维链、完整 prompt、完整 tool 参数大对象、用户图片内容、未经压缩的模型中间文本。

### 非 Agent 接口

`POST /api/user/advice-drafts/:public_id/confirm`

- 前端按钮调用。
- 校验草稿属于当前用户且 `status=draft`。
- 读取 `current_version_no` 对应内容。
- 校验 `outfit_advice.items` 中临时引用的用户已有衣物是否仍有效。
- 创建 `advices(status=ready)`。
- 更新 `advice_drafts.status=confirmed`。
- 返回正式建议 public id。

`GET /api/user/advice-drafts/current`

- 用于进入聊天页时恢复当前草稿。
- 返回当前草稿卡片数据。

`GET /api/user/advice-drafts/:public_id/versions`

- 第一版可只实现后端能力，不一定展示 UI。
- 用于后续版本历史、回滚或对比。

`POST /api/user/advice-drafts/:public_id/discard`

- 前端 `不要这版` 按钮调用。
- 校验草稿属于当前用户且 `status=draft`。
- 更新 `advice_drafts.status=discarded`。

## 状态机

草稿状态：

```text
draft
  -> confirmed
  -> discarded
```

版本行为：

```text
create_advice_draft
  -> version_no = 1

update_advice_draft
  -> version_no = current + 1
  -> 未改 JSON 分段复制上一版
  -> advice_drafts.current_version_no 更新

confirm_advice_draft
  -> 读取 current_version_no
  -> 创建 advices
  -> draft.status = confirmed
```

## 小程序交互

`pages/advisor` 聊天中新增草稿卡片消息。

卡片展示：

- 场景和日期，例如“明天见客户”。
- 版本号，例如“第 3 版”。
- 穿搭、发型、妆容三个折叠区。
- 穿搭单品按 `outfit_advice.items` 展示；已有衣物可显示引用状态，缺口单品只显示文本。
- 底部按钮：`保存为今日建议`、`继续调整`、`不要这版`。

交互：

- 点击 `继续调整`：输入框聚焦，不单独调接口。
- 用户继续输入自然语言：Agent 判断并调用 `update_advice_draft`。
- 点击 `保存为今日建议`：调用确认接口，成功后卡片状态变为“已保存”，并可跳转正式建议详情。
- 点击 `不要这版`：调用废弃接口。第一版按钮废弃不经过 Agent，避免误判。

## SSE 事件

`POST /api/user/agent/chat` 返回 SSE。

建议事件：

- `status`：处理中状态文案。
- `message`：助手完整回复文本。第一版可以不做 token 级增量。
- `draft`：草稿卡片 payload，包含 `draft_public_id`、`version_no`、三段展示内容和按钮状态。
- `done`：结束，包含 `message_public_id`、`draft_public_id`。
- `error`：错误状态和可展示文案。

Agent 决策步骤默认只写后端审计表，不直接展示给普通用户。开发或排障阶段如需前端观察，可增加仅开发环境启用的 `debug_step` SSE 事件，但不应包含隐藏思维链、完整 prompt 或敏感图片信息。

## Prompt 约束

系统提示应包含：

- 你是个人 AI 形象顾问，目标是提供中性、具体、可执行建议。
- 明星风格只能作为造型逻辑参考，不做相貌对比。
- 不承诺颜值提升。
- 不输出医疗、整形、皮肤病、植发、过敏诊断等高风险建议。
- 用户明确反馈优先于初始诊断和模型推断。
- 创建或修改建议必须使用 tools，不得假装已保存。
- 用户要求保存时，提示点击卡片按钮。
- 如果缺少必要场景信息，可以追问。
- 输出建议应包含适合原因、不建议做法和替代方案。

## 错误处理

- LLM 调用失败：写入助手错误消息，SSE 返回可展示错误。
- tool 参数校验失败：tool 返回结构化错误，Agent 可追问用户。
- 当前草稿不存在：`update_advice_draft` 返回错误，Agent 可改为创建草稿或询问是否新建。
- 草稿已确认或废弃：禁止更新，提示用户新建方案。
- 确认接口重复调用：如果已确认，直接返回已有 `confirmed_advice_id`，保持幂等。
- 确认保存时衣物引用不存在或不属于当前用户：不写入该正式引用，返回可展示提示或让用户回到草稿继续调整。

## 测试策略

服务端：

- Agent route：SSE 事件包含 `message`、`draft`、`done`。
- Agent run 测试：每条用户消息创建一条助手占位 `chat_msgs`，完成后更新状态、文本和业务关联。
- Agent step 测试：模型决策、tool 调用、tool 结果和最终回复按顺序写入 `agent_run_steps`。
- Agent step 失败测试：tool 失败时记录 `failed` 步骤和错误摘要，不丢失已完成步骤。
- tool 单元测试：创建草稿、精修草稿、复制未修改 JSON 分段、废弃草稿。
- repository 测试：版本号唯一、当前版本查询、用户隔离。
- 确认接口测试：从最新版生成正式 `advices`，重复确认幂等。
- 确认接口测试：校验 `outfit_advice.items` 中已有衣物引用失效时不写入正式引用，并返回可展示提示。
- 权限测试：不能读取或修改其他用户草稿。

小程序：

- `advisor` 页面可渲染草稿卡片。
- 保存按钮调用确认接口。
- 继续调整只聚焦输入框。
- SSE `draft` 事件能更新卡片。

验证命令：

- `go test ./...`
- `npm --prefix miniapp run verify` 或相关页面脚本。

## 后续扩展

- 多草稿并列和“改第一套/第二套”。
- 草稿版本对比、回滚和恢复。
- 将用户对草稿的采纳、拒绝、修改反馈沉淀为结构化记忆。
- 在正式建议详情页展示版本来源。
- 将热门修改点用于优化系统提示和工具字段。
