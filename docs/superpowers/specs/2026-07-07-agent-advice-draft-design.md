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
- 草稿版本使用独立表和明确字段，尽量避免 JSON 垃圾堆。
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

保留 `advice_requests` 作为原始请求记录。新增草稿体系承载可对话精修的内容和版本。

```text
chat_msgs(user)
  -> chat_msgs(assistant)
       -> agent_run_steps(assistant_msg_id)
  -> advice_requests(source_msg_id)
  -> advice_drafts(advice_request_id)
  -> advice_draft_versions(draft_id)
       -> advice_draft_outfits
       -> advice_draft_hairs
       -> advice_draft_makeups
       -> advice_draft_wardrobe_refs
```

### `advice_requests`

继续记录一次建议请求的来源和原始输入。

关键字段沿用已有设计：

- `source`
- `source_msg_id`
- `status`
- `input_text`
- `input_assets`
- `scenario`
- `trigger_context`
- `parent_request_id`
- `parent_advice_id`

约束：

- 不存草稿正文。
- `status` 可扩展为 `drafting`、`confirmed`、`discarded`。
- `scenario` 和 `trigger_context` 仅存请求级上下文，不承担版本历史。

### `chat_msgs`

聊天消息表同时承载用户可见消息和一次 Agent 执行的头记录。

关键字段和扩展字段：

- `id`
- `public_id`
- `user_id`
- `source_msg_id`：新增可空字段，仅助手消息用于指向触发本轮 Agent 的用户消息
- `role`：`user` / `assistant`
- `msg_type`：`text` / `draft_card` / `error`
- `content_text`
- `content_json`
- `asset_refs`
- `related_type`
- `related_id`
- `related_public_id`
- `job_id`
- `status`

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

- `id`
- `public_id`
- `user_id`
- `source_msg_id`
- `assistant_msg_id`
- `step_no`
- `step_type`：`model_decision` / `tool_call` / `tool_result` / `final_response` / `error`
- `status`：`running` / `succeeded` / `failed` / `skipped`
- `usage_key`
- `provider_code`
- `model_code`
- `prompt_version`
- `max_step`
- `tool_name`
- `tool_call_id`
- `decision_label`：`answer` / `create_draft` / `update_draft` / `discard_draft` / `read_context` / `ask_clarification`
- `input_summary`
- `output_summary`
- `related_type`：`advice_draft` / `advice_draft_version` / `chat_msg` / `memory` / `clothes_item`
- `related_id`
- `related_public_id`
- `started_at`
- `finished_at`
- `duration_ms`
- `error_message`
- `created_at`

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

- `id`
- `public_id`
- `advice_request_id`
- `user_id`
- `source_msg_id`
- `status`：`draft` / `confirmed` / `discarded`
- `scene_key`
- `scene_label`
- `target_date`
- `occasion`
- `weather_text`
- `mood_text`
- `style_goal`
- `avoid_goal`
- `current_version_no`
- `confirmed_advice_id`
- `created_at`
- `updated_at`
- `deleted_at`

约束：

- 当前草稿通过 `user_id + status=draft + updated_at` 查询。
- 第一版每个用户在 advisor 入口当前只操作最近一个 `draft` 草稿。
- `confirmed_advice_id` 在确认保存后写入。

### `advice_draft_versions`

草稿版本表，每轮创建或精修都新增一条。

字段：

- `id`
- `public_id`
- `draft_id`
- `user_id`
- `version_no`
- `source_msg_id`
- `user_intent`
- `revision_summary`
- `changed_outfit`
- `changed_hair`
- `changed_makeup`
- `created_at`

约束：

- `(draft_id, version_no)` 唯一。
- 当前版本由 `advice_drafts.current_version_no` 指向。
- 每个版本必须有穿搭、发型、妆容三段内容记录。未修改分段从上一版复制。

### `advice_draft_outfits`

穿搭草稿表。

字段：

- `id`
- `draft_version_id`
- `title`
- `summary`
- `top_item_text`
- `bottom_item_text`
- `outer_item_text`
- `shoe_item_text`
- `bag_item_text`
- `accessory_text`
- `color_strategy`
- `silhouette_strategy`
- `material_strategy`
- `why_text`
- `avoid_text`
- `alternative_text`
- `created_at`

### `advice_draft_hairs`

发型草稿表。

字段：

- `id`
- `draft_version_id`
- `title`
- `length_direction`
- `shape_direction`
- `bangs_direction`
- `styling_steps`
- `hold_level`
- `why_text`
- `avoid_text`
- `alternative_text`
- `created_at`

### `advice_draft_makeups`

妆容草稿表。

字段：

- `id`
- `draft_version_id`
- `title`
- `base_direction`
- `brow_direction`
- `eye_direction`
- `lip_direction`
- `cheek_direction`
- `finish_level`
- `step_text`
- `why_text`
- `avoid_text`
- `alternative_text`
- `safety_note`
- `created_at`

妆容边界：

- 只做场景表达、色彩、浓淡、步骤和避雷。
- 不做肤质诊断、过敏判断、治疗建议或医疗结论。

### `advice_draft_wardrobe_refs`

草稿引用衣物表。

字段：

- `id`
- `draft_version_id`
- `clothes_item_id`
- `clothes_item_public_id`
- `role`：`top` / `bottom` / `outer` / `shoe` / `bag` / `accessory`
- `reason_text`
- `created_at`

用途：

- 查询哪些衣服被建议过。
- 支持用户后续反馈“这件不要再推荐”。
- 为正式建议生成 `wardrobe_item_refs` 提供结构化来源。

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
- 返回场景字段、版本号、三段建议和衣物引用。

### 写草稿 tools

`create_advice_draft`

- 创建 `advice_requests`。
- 创建 `advice_drafts(status=draft)`。
- 创建 `advice_draft_versions(version_no=1)`。
- 创建穿搭、发型、妆容和衣物引用记录。
- 返回草稿 public id、版本号和可渲染卡片数据。

`update_advice_draft`

- 读取当前用户自己的 `status=draft` 草稿。
- 基于当前版本创建 `version_no + 1`。
- 修改指定分段，未修改分段从上一版复制。
- 更新 `advice_drafts.current_version_no`。
- 返回新版草稿卡片数据。

`discard_advice_draft`

- 仅废弃当前用户自己的 `status=draft` 草稿。
- 将 `advice_drafts.status` 改为 `discarded`。
- 将关联 `advice_requests.status` 改为 `discarded`。

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
- 创建 `advices(status=ready)`。
- 更新 `advice_drafts.status=confirmed`。
- 更新 `advice_requests.status=confirmed`。
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
- 更新 `advice_requests.status=discarded`。

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
  -> 未改分段复制上一版
  -> advice_drafts.current_version_no 更新

confirm_advice_draft
  -> 读取 current_version_no
  -> 创建 advices
  -> draft.status = confirmed
  -> request.status = confirmed
```

## 小程序交互

`pages/advisor` 聊天中新增草稿卡片消息。

卡片展示：

- 场景和日期，例如“明天见客户”。
- 版本号，例如“第 3 版”。
- 穿搭、发型、妆容三个折叠区。
- 关联衣物名称和角色。
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
- 衣物引用不存在或不属于当前用户：拒绝写入该引用，并让 Agent 改用文本描述或重新查询。

## 测试策略

服务端：

- Agent route：SSE 事件包含 `message`、`draft`、`done`。
- Agent run 测试：每条用户消息创建一条助手占位 `chat_msgs`，完成后更新状态、文本和业务关联。
- Agent step 测试：模型决策、tool 调用、tool 结果和最终回复按顺序写入 `agent_run_steps`。
- Agent step 失败测试：tool 失败时记录 `failed` 步骤和错误摘要，不丢失已完成步骤。
- tool 单元测试：创建草稿、精修草稿、复制未修改分段、废弃草稿。
- repository 测试：版本号唯一、当前版本查询、用户隔离。
- 确认接口测试：从最新版生成正式 `advices`，重复确认幂等。
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
