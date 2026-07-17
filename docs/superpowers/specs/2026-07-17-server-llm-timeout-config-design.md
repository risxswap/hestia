# 服务端大模型超时配置设计

## 背景

Agent 聊天当前存在两层固定超时：Agent 整轮执行上限为 15 秒，模型 HTTP 请求上限为 60 秒。外层超时短于内层超时，导致模型请求仍处于正常等待阶段时，Agent 上下文已经被取消，并产生 `context deadline exceeded`。

## 目标

- 将 Agent 整轮执行超时和模型 HTTP 请求超时改为服务端配置项。
- 默认配置保证 Agent 总超时大于单次模型请求超时。
- 非法配置在服务启动阶段明确报错，不静默使用错误值。
- 保持未设置新环境变量时的部署兼容性。

## 非目标

- 不新增模型请求重试。
- 不改变 Agent 最大迭代次数。
- 不调整供应商、模型或业务 usage 配置。
- 不修改小程序请求超时与交互行为。
- 不记录完整请求 URL、鉴权信息、提示词或模型响应。

## 配置契约

新增两个按秒配置的环境变量：

| 环境变量 | 默认值 | 用途 |
| --- | ---: | --- |
| `AGENT_RUNNER_TIMEOUT_SECONDS` | 75 | Agent 整轮执行上限，包括模型决策和工具调用 |
| `LLM_REQUEST_TIMEOUT_SECONDS` | 60 | 单次模型 HTTP 请求上限 |

配置加载后转换为 `time.Duration` 使用。两个值必须为正整数；显式配置为零、负数或非整数时，配置加载直接返回错误。为了避免外层提前取消内层请求，`AGENT_RUNNER_TIMEOUT_SECONDS` 必须大于 `LLM_REQUEST_TIMEOUT_SECONDS`，不满足时服务拒绝启动。

## 实现方案

### 配置加载

在 `infra/config.Config` 增加两个秒数配置字段，通过现有 `env` 解析器提供默认值与环境变量覆盖。配置加载完成后统一校验正数及大小关系。

### Agent 超时

Agent Service 保存构造时注入的执行超时。路由装配从全局配置读取 `AGENT_RUNNER_TIMEOUT_SECONDS`，转换为 `time.Duration` 后传入 Service。执行 `AdviceRunner.Run` 时使用该值创建超时 Context，不再引用包级固定 15 秒常量。

测试或旧构造路径未显式传入时使用 75 秒默认值，避免现有调用方行为异常。

### 模型请求超时

LLM Service 保存构造时注入的请求超时。路由装配从全局配置读取 `LLM_REQUEST_TIMEOUT_SECONDS` 并传入 LLM Service。Qwen 与 OpenAI 兼容客户端配置均读取该值，不再固定为 60 秒。

测试或旧构造路径未显式传入时使用 60 秒默认值，保持构造 API 向后兼容。

### Agent 失败诊断日志

Agent Runner 失败时，在现有错误摘要之外增加结构化诊断字段：

- `provider_code`、`model_code`：实际解析出的供应商和模型。
- `provider_host`：仅记录 URL 主机名，不记录路径、查询参数或签名信息。
- `agent_timeout_ms`、`llm_timeout_ms`：本轮生效的两层超时。
- `context_deadline`：是否存在截止时间及其 RFC3339 时间。
- `context_error`：`context.Cause(ctx)` 的分类摘要，例如 `deadline_exceeded` 或 `canceled`。
- `error_type`：最外层 Go 错误的类型名称。
- `error_chain`：按包装顺序记录脱敏后的错误类型和摘要，便于识别 `NodeRunError`、HTTP 错误和 context 错误之间的关系。
- `node_path`：从 Eino 错误中提取的节点路径，例如 `node_1, ChatModel`。
- `timeout_source`：根据生效 Context 和错误链归类为 `agent_context`、`llm_client`、`http_transport` 或 `unknown`。

错误字段继续通过现有日志脱敏器处理。URL 只提取主机名；错误链不得包含 API Key、Bearer Token、完整请求正文或图片 URL。字段提取失败时写入空值或 `unknown`，不能阻止业务兜底响应。

## 错误处理

- 缺少环境变量时使用默认值，不报错。
- 非整数由现有环境变量解析器返回配置错误。
- 零值、负数或 Agent 超时不大于模型超时时，由 `config.Load` 返回包含配置项名称的错误。
- 运行期模型超时继续沿用现有错误日志和用户兜底响应，不改变 API 协议。

## 测试与验证

按测试驱动方式覆盖：

- 未设置环境变量时加载 `75 / 60` 默认值。
- 环境变量可以覆盖两个默认值。
- 零值、负数、非整数配置加载失败。
- Agent 超时不大于模型超时时配置加载失败。
- Agent Service 使用注入值创建执行 Context。
- Qwen 与 OpenAI 兼容模型客户端均使用注入的请求超时。
- 原有构造路径继续使用默认值。
- Agent 超时错误日志包含上述诊断字段，并且不泄露敏感值。

完成后运行相关包测试与服务端全量 `go test ./...`。

## 验收标准

- 聊天请求不再固定于 15 秒被 Agent 外层取消。
- 两层超时均可通过环境变量独立调整。
- 默认配置下 Agent 总超时为 75 秒，模型单次请求超时为 60 秒。
- 错误的超时层级或非法数值会阻止服务启动并提供明确错误。
- `context deadline exceeded` 日志可以区分 Agent 外层超时与模型 HTTP 层超时，并定位 Eino 节点路径。
- 现有服务端测试及新增配置测试通过。
