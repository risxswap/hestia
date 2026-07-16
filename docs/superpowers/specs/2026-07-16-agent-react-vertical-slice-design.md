# Agent ReAct 端到端纵切设计

## 背景

当前智能体虽然使用 Eino ADK 和工具调用，但仍存在三个直接影响用户感知的问题：

- 聊天图片只以素材 ID 文本进入 prompt，模型没有真正观察图片。
- 最终回复要求模型自行声明已经执行的工具动作，审计记录不来自 ADK 原生事件。
- system prompt 固化在 Go 代码中，工具调用时机描述较弱，无法动态调整 Agent 策略。

本次改造不考虑旧链路兼容，以最新设计为准，将智能体收敛为标准 ReAct Agent。Agent 自主判断是否追问、回答或调用工具；档案、记忆、衣橱、图片分析和草稿操作都作为工具提供。后端不实现隐式意图路由或固定工作流，只保留权限、参数、状态、安全、超时和额度等硬边界。

## 目标

本次只完成第一条端到端纵切：

1. Agent 可以主动调用视觉工具，真正理解用户上传的自拍、穿搭图、衣橱图或参考图。
2. Agent 使用标准 ReAct 循环，自主选择读取上下文和操作草稿的工具。
3. 工具调用和结果直接来自 ADK 原生事件，不再依赖模型在最终回复中自报。
4. 小程序展示经过脱敏的真实处理过程，例如查看照片、读取档案、检索记忆和查看衣物。
5. system prompt 通过 `system_configs` 动态更新，使用 XML 分块组织。

本次不实现长期记忆写回、反馈强化、记忆语义检索和 prompt 管理后台。

## 核心原则

- Agent 拥有决策自由：普通聊天可以零工具完成，复杂任务可以连续调用多个工具。
- 后端不替 Agent 判断业务意图，不预取档案、记忆、衣橱或图片分析结果。
- 所有业务事实以真实工具执行结果为准，模型文本不能声明或伪造工具执行状态。
- 不记录模型隐藏思维链，只记录工具调用、工具结果、最终回复和必要的运行元数据。
- 图片原始内容只在视觉工具内部短暂使用，不进入聊天记录或审计表。
- 用户权限、敏感数据保护和高风险表达边界不能交给模型自行决定。

## 总体架构

```text
用户消息 + 最近对话 + 当前草稿摘要 + 素材引用
  -> 请求级 Agent Factory
       -> 读取当前模型配置
       -> 读取 system_configs(agent.prompts/system)
       -> 创建 ReAct Agent 和稳定工具集
  -> ReAct Loop
       Thought: 模型内部判断目标和信息缺口
       Action: 自主选择工具
       Observation: 工具返回结构化结果
       重复 Action / Observation
  -> Final Answer
       -> 助手自然语言回复
       -> 可选草稿卡片
  -> ADK 原生事件审计
       -> agent_run_steps
       -> 脱敏处理过程
```

`Thought` 只存在于模型内部，不持久化、不输出给用户。系统只消费可以观察到的 tool call、tool result 和 final response。

## 请求级 Agent Factory

现有 Agent 在路由注册时创建一次，无法及时应用动态 prompt 和模型配置。新版每个聊天请求创建本轮 Runner：

1. 从 `system_configs` 读取唯一 system prompt。
2. 从 LLM 配置解析 `agent_chat` 的文本、工具调用模型。
3. 注册固定工具集。
4. 使用当前 system prompt 创建 Eino `ChatModelAgent`。
5. 设置最大迭代次数、请求超时和本轮审计元数据。

模型或 prompt 配置失败时，本轮请求明确失败并写结构化日志，不再静默退化为固定回复。

## System Prompt 配置

system prompt 复用现有 `system_configs`：

```text
group      = agent.prompts
key        = system
value_type = string
status     = active
value      = "<agent>...</agent>"
```

`system_configs.value` 是 JSON 字段，因此数据库中保存 JSON 字符串，读取后得到 XML 原文。

约束：

- 只有这一条 system prompt，不设置版本字段。
- 每次请求直接读取当前有效内容，不使用进程内缓存或代码内默认 prompt。
- 配置缺失、非激活、内容为空、XML 解析失败或缺少必要分块时，请求失败。
- 日志可以记录 prompt 内容 SHA-256 的短摘要用于排障，但摘要不是业务版本。
- 现有 `agent_run_steps.prompt_version` 不再写入，新链路不依赖它。

## System Prompt XML 结构

首版 system prompt 使用以下稳定分块：

```xml
<agent>
  <identity>
    你是 Hestia，持续了解用户的个人 AI 形象顾问。
    你的目标是提供中性、具体、可执行、可调整的建议。
  </identity>

  <operating_mode>
    使用 ReAct 工作：理解目标，判断信息是否充分，自主选择工具，
    根据工具观察继续行动，直到可以回答或需要向用户追问。
    不向用户展示隐藏推理过程。
  </operating_mode>

  <decision_principles>
    <principle priority="1">用户本轮明确要求优先。</principle>
    <principle priority="2">用户确认的事实和真实反馈优先于 AI 推断。</principle>
    <principle priority="3">信息不足且不同答案会显著改变建议时，先追问。</principle>
    <principle priority="4">普通聊天或常识回答不必调用工具。</principle>
    <principle priority="5">不要为了展示能力而调用无关工具。</principle>
  </decision_principles>

  <tool_policy>
    <inspect_user_image>回答依赖图片中可观察的信息时调用。</inspect_user_image>
    <get_profile_context>建议依赖稳定档案、生活场景或风格目标时调用。</get_profile_context>
    <get_memory_context>建议依赖偏好、禁忌、历史反馈或过去信息时调用。</get_memory_context>
    <get_wardrobe_context>用户要求使用现有衣物或引用具体衣物时调用。</get_wardrobe_context>
    <get_current_advice_draft>修改、解释、比较、保存或放弃当前方案时调用。</get_current_advice_draft>
    <create_advice_draft>用户需要完整可调整方案且信息充分时调用。</create_advice_draft>
    <update_advice_draft>用户明确修改已有草稿时调用，并先读取当前草稿。</update_advice_draft>
    <discard_advice_draft>用户明确放弃当前草稿时调用，意图不清时先确认。</discard_advice_draft>
  </tool_policy>

  <image_boundaries>
    只描述可观察的造型、比例、廓形、色彩、发型和单品结构。
    区分观察事实与推断，不做人脸身份识别、相貌匹配、医疗判断或羞辱性评价。
  </image_boundaries>

  <response_contract>
    默认自然对话，不输出 JSON、工具名或内部步骤。
    建议说明适合原因、不建议项和替代方案；工具失败时如实说明。
  </response_contract>
</agent>
```

服务端校验 `identity`、`operating_mode`、`decision_principles`、`tool_policy`、`image_boundaries` 和 `response_contract` 六个一级分块存在。具体文案可动态更新。

## 工具暴露策略

主 Agent 始终暴露同一组 8 个工具，不根据当前草稿或请求内容动态删减工具：

1. `inspect_user_image`
2. `get_profile_context`
3. `get_memory_context`
4. `get_wardrobe_context`
5. `get_current_advice_draft`
6. `create_advice_draft`
7. `update_advice_draft`
8. `discard_advice_draft`

稳定工具集合让模型形成一致的选择策略。无效状态由工具返回结构化业务错误，不由后端提前隐藏能力。

工具 schema 由 Go 类型生成，是参数契约的唯一事实来源。system prompt 负责跨工具决策原则和调用时机；工具 description 只说明自身用途、适用时机、返回内容和关键限制，避免重复整份 policy。

第一版不提供任意 SQL、任意 HTTP、任意文件读取或通用代码执行工具。

## 通用工具结果

工具业务错误统一返回可供 Agent 判断的结构：

```json
{
  "ok": false,
  "error_code": "asset_not_found",
  "message": "无法读取指定图片",
  "retryable": false
}
```

内部数据库错误、对象存储密钥、签名 URL 和堆栈不得进入工具结果。工具失败作为 Observation 返回 Agent，由 Agent 自主决定修正参数、调用其他工具、追问或向用户解释。

## 视觉工具

### 输入

```json
{
  "asset_public_ids": ["ast_xxx"],
  "analysis_focus": "分析当前穿搭的比例和可调整点"
}
```

约束：

- `asset_public_ids` 至少一个，数量和总大小受服务端限制。
- 只接受当前用户有权访问、状态正常且 MIME 为支持图片类型的素材。
- 参数不接受 URL、对象存储 key 或任意外部地址。

### 执行

1. 根据当前工具会话取得可信 `user_id`。
2. 查询并校验素材归属、状态、MIME 和用途。
3. 在工具内部生成短期私有读取 URL，或由服务端读取为视觉模型支持的输入。
4. 使用独立的 `image_analysis` LLM usage 调用视觉模型。
5. 将模型结果解析为结构化观察并返回主 Agent。

如果没有满足视觉能力的模型、素材无权访问、图片不可读取或分析失败，工具返回明确错误。主 Agent 不得收到不可用图片的伪观察结果。

### 输出

```json
{
  "ok": true,
  "images": [
    {
      "asset_public_id": "ast_xxx",
      "observations": [
        {
          "dimension": "outfit",
          "statement": "上装长度接近胯部",
          "confidence": 0.88
        }
      ],
      "limitations": ["照片未包含鞋子区域"]
    }
  ]
}
```

工具只返回图片中可观察的形象信息，并显式返回观察局限。禁止身份识别、明星相貌匹配、医疗或病理判断、敏感属性推断和羞辱性描述。

## 其他工具调用时机

- `get_profile_context`：建议需要结合稳定身体档案、生活场景或风格目标时调用。
- `get_memory_context`：建议需要结合偏好、禁忌、历史反馈，或用户说“你应该记得”时调用。查询参数使用短关键词，不复制整句问题。
- `get_wardrobe_context`：用户要求使用现有衣物，或方案需要引用具体衣物时调用。
- `get_current_advice_draft`：用户要求继续修改、解释、比较、保存或放弃当前方案时调用。
- `create_advice_draft`：信息充分且用户确实需要一套可继续调整或保存的完整方案时调用。普通问答、寒暄和单点建议不创建草稿。
- `update_advice_draft`：用户明确修改已有草稿时调用。system prompt 要求先读取当前草稿，但后端不实现前置守卫。
- `discard_advice_draft`：用户明确放弃当前草稿时调用；意图不明确时先追问。

## ReAct 输入与最终回复

Agent 初始输入只包含：

- 当前用户文本。
- 最近聊天消息。
- 当前草稿轻量摘要。
- 本轮素材引用的 ID、类型和用户备注。

图片二进制、签名 URL、完整档案、全量记忆和全量衣橱不默认进入 prompt。Agent 需要时通过工具读取。

最终回复使用自然语言，不再要求 JSON 包装，也不包含 `tool_calls` 自报字段。草稿业务状态来自真实写工具结果；Runner 根据原生工具结果关联并刷新草稿卡片。

## ADK 原生事件审计

Runner 必须直接消费 ADK 原生事件并转换为审计步骤：

| 原生事件 | `agent_run_steps.step_type` | 记录内容 |
| --- | --- | --- |
| 模型发起工具调用 | `tool_call` | 工具名、调用 ID、短输入摘要、开始时间 |
| 工具成功返回 | `tool_result` / `succeeded` | 工具名、调用 ID、短输出摘要、耗时、业务关联 |
| 工具失败返回 | `tool_result` / `failed` | 工具名、调用 ID、安全错误摘要、耗时 |
| 最终模型回复 | `final_response` | 用户可见回复摘要和完成时间 |

不再根据最终模型文本合成工具审计，不再保存模型声明的 `tool_calls`。不记录 Thought、隐藏思维链、完整工具参数、完整工具返回、图片内容或签名 URL。

## 用户可见处理过程

小程序继续使用现有步骤展示能力，但展示内容来自真实审计事件。工具名映射为固定脱敏文案：

- `inspect_user_image`：查看你上传的照片
- `get_profile_context`：读取个人档案
- `get_memory_context`：检索长期记忆
- `get_wardrobe_context`：查看核心衣物
- `get_current_advice_draft`：读取当前方案
- `create_advice_draft`：生成建议草稿
- `update_advice_draft`：调整建议草稿
- `discard_advice_draft`：放弃当前草稿

前端不展示工具参数、原始返回、模型 Thought 或内部错误。失败步骤显示中性说明，例如“未能读取照片”，最终回复由 Agent 根据 Observation 解释下一步。

## 错误处理

### 工具业务错误

作为结构化 Observation 返回 Agent，允许同一 ReAct 运行继续。Agent 可以重试、换工具、追问或结束回复。

### Agent 运行错误

超时、达到最大迭代次数、模型请求失败或事件流中断时，本轮助手消息标记失败，保留此前已经发生的真实审计步骤，并返回明确错误，不生成伪成功草稿。

### 初始化错误

system prompt、模型配置或工具创建失败时直接拒绝本轮请求，记录结构化服务端日志。不得静默使用规则 Runner 或固定建议作为成功响应。

## 测试与评测

### 自动化测试

- Prompt 配置：唯一 key 读取、状态校验、空值、非法 XML、缺少分块。
- 视觉工具：用户权限、素材状态、MIME、视觉模型能力、结构化输出和安全错误。
- 工具契约：统一错误结构、会话用户绑定、业务状态错误。
- ReAct Runner：真实 tool call、tool result、失败结果和 final response 的事件顺序及关联 ID。
- Service：审计持久化、助手消息状态、草稿卡片关联、运行失败保留步骤。
- 小程序：新增读取工具的脱敏步骤文案和失败展示。

### Policy 固定案例

至少覆盖：

1. 普通寒暄不调用工具。
2. 用户要求分析自拍时调用 `inspect_user_image`。
3. 用户要求结合个人情况时读取档案和相关记忆。
4. 用户要求用现有衣物搭配时读取衣橱。
5. 用户修改已有草稿时先读取草稿再更新。
6. 图片工具失败时不假装已经看图。
7. 单点知识建议不自动创建完整草稿。

Policy 评测用于判断 system prompt 和模型选择是否可靠，不转化为后端固定工作流。

## 删除和替换的旧设计

本次不考虑兼容，以下旧行为直接删除：

- 固化在 `adk_runner.go` 中的 system prompt。
- 最终 JSON 的 `tool_calls` 自报协议和对应解析结构。
- 根据模型自报工具动作合成审计步骤的逻辑。
- 图片素材 ID 作为文本替代真实视觉观察的路径。
- Agent 初始化失败后静默保留无 Runner 服务的路径。
- 规则 Runner 参与线上 Agent 决策或降级的路径。

## 验收标准

- 用户要求分析上传图片时，审计中存在真实 `inspect_user_image` 调用和结果；视觉不可用时明确失败，不生成图片结论。
- 普通文本聊天可以不调用任何工具完成。
- Agent 能在一轮内连续读取多个上下文工具并基于 Observation 继续行动。
- 草稿状态只由真实写工具执行结果决定。
- `agent_run_steps` 中的工具步骤与 ADK 原生事件一一对应，不再来自最终回复文本。
- 小程序展示真实、脱敏的读取和写入步骤。
- 更新 `system_configs(agent.prompts/system)` 后，新请求使用更新后的 XML prompt，无需重新编译或重启服务。
- system prompt 或模型配置无效时请求明确失败，服务端日志包含可定位但不泄露敏感数据的信息。
