# 服务端与大模型调用日志增强设计

## 背景

服务端目前使用 Go `slog` 输出 JSON 日志。普通大模型生成已经记录开始、完成和部分失败信息，但仍存在以下缺口：

- LLM 配置解析失败和客户端不可用会提前返回，没有异常日志。
- Agent 使用 Tool Calling 模型，不经过普通 `Generate` 调用的完整日志链路。
- HTTP 层没有统一的请求日志、请求标识和 panic 恢复日志。
- 少数业务 handler 允许 logger 为空，异常可能只转成 HTTP 错误而不输出。
- 当前日志缺少输入规模、消息角色分布、提示词版本等排障上下文。

本次只增强服务端日志，不修改小程序端。重点保证大模型调用的成功、失败和初始化异常均可定位，同时不泄露用户敏感数据。

## 目标

- 每个 HTTP 请求具有可关联的 `request_id`，并记录请求结果与耗时。
- 未捕获 panic 必须输出异常值和堆栈，并向客户端返回统一 500 响应。
- 普通 LLM 生成和 Agent Tool Calling 调用均记录开始、完成和失败事件。
- 配置解析、模型初始化、模型执行等阶段的异常必须以 `ERROR` 级别记录。
- 日志提供足够的模型、输入规模、角色分布和耗时上下文。
- 提示词、模型原始响应、密钥、图片地址等敏感内容不得完整写入日志。

## 非目标

- 不接入外部可观测平台或分布式追踪系统。
- 不记录完整提示词、完整模型响应、API Key、签名 URL 或图片内容。
- 不修改业务 API 响应结构和现有错误码。
- 不为小程序端补日志。
- 不在本次改动中引入完整的 Eino callback 追踪体系。

## 总体方案

采用两层日志设计：

1. HTTP 中间件提供请求级关联、访问日志和 panic 兜底。
2. LLM 服务与 Agent Runner 提供模型调用级日志，覆盖普通生成和 Tool Calling 两条路径。

业务 handler 继续记录具有业务语义的失败信息。统一中间件不重复猜测 handler 内部错误，只负责请求结果和未捕获 panic。

## HTTP 日志中间件

在用户 API Router 上注册统一中间件，并使用依赖中的 logger；依赖或 logger 为空时回退到 `slog.Default()`，避免静默跳过日志。

### 请求标识

- 优先接受合法的 `X-Request-ID` 请求头。
- 缺失或不合法时生成新的随机标识。
- 将 `request_id` 写入 Gin Context、请求 Context 和响应头。
- LLM 日志从 Context 读取同一个标识，实现请求与模型调用关联。

### 访问日志

请求完成后输出一条结构化日志，字段包括：

- `request_id`
- `method`
- `route`
- `status_code`
- `duration_ms`
- `client_ip`

正常请求使用 `INFO`；4xx 使用 `WARN`；5xx 使用 `ERROR`。不记录查询参数、请求体、Authorization、Cookie 或响应体。

### panic 日志

Recovery 中间件捕获 panic 后必须：

- 以 `ERROR` 输出 `request_id`、method、route、panic 值和堆栈。
- 中止后续 handler。
- 若响应尚未写出，返回现有统一格式的 500 错误。

panic 值可能携带敏感正文，因此只写经过脱敏和截断的摘要；堆栈保留函数与源码定位信息。

## LLM 调用日志

### 覆盖范围

以下阶段均纳入日志：

- LLM 客户端不可用。
- usage/provider/model 配置解析。
- 普通 Chat Model 初始化。
- Tool Calling Chat Model 初始化。
- 普通模型 Generate 调用。
- Agent Runner 中的模型执行。

每次模型执行使用独立的 `llm_call_id`。同一 HTTP 请求内可以有多个模型调用，它们共享 `request_id`。

### 事件与级别

- `llm call started`：`INFO`。
- `llm call completed`：`INFO`。
- `llm call failed`：`ERROR`，必须携带 `error` 和失败阶段。
- 初始化前即失败的情况也输出 `llm call failed`，不得直接返回而不记录。

失败阶段使用稳定值：`client_check`、`config_resolve`、`model_init`、`model_generate`、`agent_run`。

### 公共字段

- `request_id`
- `llm_call_id`
- `usage_key`
- `provider_code`
- `model_code`
- `prompt_version`
- `required_caps`
- `message_count`
- `system_message_count`
- `user_message_count`
- `assistant_message_count`
- `tool_message_count`
- `input_chars`
- `image_count`
- `param_keys`
- `duration_ms`
- `output_chars`
- `input_summary`
- `output_summary`
- `error_stage`
- `error`

尚未解析出 provider、model 或 prompt version 时省略对应字段，不写空的误导值。`param_keys` 只记录排序后的参数名，不记录参数值。

### 脱敏摘要

摘要用于判断输入和输出的大致内容，不承担审计或复现职责。

- 分别从文本输入和模型输出生成摘要。
- 统一折叠空白并限制长度，超出部分标记为已截断。
- 过滤 URL、邮箱、手机号、Bearer/Token/API Key 等常见敏感模式。
- 图片只记录数量，不记录 URL、对象键或签名参数。
- 不记录原始消息数组、完整提示词、完整模型响应或工具参数正文。
- 错误信息同样经过长度限制和敏感模式过滤后再输出。

脱敏函数放在服务端公共日志模块，供 HTTP panic 和 LLM 日志复用，并保持确定性以便测试。

## Agent Tool Calling

当前 Tool Calling 模型在路由装配阶段创建，实际调用发生在 Agent Runner。仅增强普通 `llm.Service.Generate` 无法覆盖这条链路，因此：

- 模型创建阶段由 LLM Service 记录配置解析和初始化错误。
- Agent Runner 执行模型前后记录开始、完成、失败。
- 已有 `agent_run_steps` 审计继续保留；结构化运行审计与运行日志职责不同，不互相替代。
- 工具调用日志只记录工具名、调用标识、耗时、状态和脱敏摘要，不记录完整工具输入输出。

## 错误处理原则

- 所有被转换、包装或返回给上层的异常，必须在最接近失败事实且具备完整上下文的一层打印一次。
- 上层可以记录业务失败结果，但不重复输出同一份堆栈。
- `ERROR` 日志始终携带 Go `error` 值或 panic 摘要，并包含稳定的失败阶段。
- logger 不得因为依赖为空而失效，统一回退到 `slog.Default()`。
- 日志失败不改变业务结果，也不向客户端暴露内部异常。

## 测试与验证

按测试驱动方式覆盖：

- HTTP 请求生成、透传并回写 `request_id`。
- HTTP 请求日志包含 method、route、status 和耗时，不包含请求正文或鉴权信息。
- panic 输出 `ERROR` 与堆栈，并返回统一 500。
- 普通 LLM 调用记录开始、完成、模型信息、角色统计、字符数和耗时。
- 配置解析、模型初始化、模型生成失败分别记录对应 `error_stage`。
- Tool Calling 初始化失败必定记录异常。
- Agent Runner 的模型失败必定记录异常。
- logger 为空时仍通过默认 logger 输出异常。
- 提示词、响应、API Key、图片 URL、token、手机号和邮箱不会原样出现在日志。
- 脱敏摘要长度固定受限，输出具有确定性。

完成后运行相关 Go 单元测试及服务端全量测试；若环境依赖导致集成测试无法执行，需要明确说明。

## 验收标准

- 任意服务端未捕获 panic 都能通过一条带堆栈的 `ERROR` 日志定位。
- 普通生成与 Agent Tool Calling 的每次模型执行均有可关联的开始和结束事件。
- LLM 任一失败阶段至少产生一条包含 `request_id`、`llm_call_id`、失败阶段和错误的 `ERROR` 日志。
- 日志中不出现完整提示词、完整模型响应、API Key、图片签名 URL 或鉴权信息。
- 原有 API 行为和响应格式保持兼容，相关测试全部通过。
