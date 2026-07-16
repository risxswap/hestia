# Agent 聊天处理过程与恢复计时实现计划

> **执行说明：** 按照 `superpowers:executing-plans` 与 TDD 流程逐项实施。每个行为先添加失败测试，再补最小实现。

**目标：** 在顾问聊天中通过 SSE 显示会随 Agent 阶段变化的处理摘要，支持展开查看脱敏步骤；服务端持久化助理回复开始时间，使页面刷新或 SSE 重连后的计时能从正确时刻继续。

**架构：** `chat_msgs.created_at` 是助理回复处理的权威开始时间。服务层在创建助理占位消息后产生用户安全的 `ChatProcessEvent`，HTTP handler 将它编码为 SSE `process` 事件。`agent_run_steps` 继续记录完整审计数据；新增会话恢复查询时仅将由步骤类型和工具名映射出的安全摘要返回给前端。小程序依据 `response_started_at` 本地计算经过秒数，不把计时状态作为客户端事实保存。

**技术栈：** Go、Gin、sqlx/MySQL、Eino ADK、微信小程序原生页面与已有请求封装。

## 范围与约束

- 不增加 `agent_runs` 或新表。
- 不改变 `chat_msgs`（用户可见消息）与 `agent_run_steps`（后台审计）的职责分界。
- 不传递 `input_summary`、`output_summary`、工具参数、模型配置、原始错误或内部决策标签至客户端。
- 对进行中的消息，恢复后以 `chat_msgs.created_at` 继续前端计时；不保存浏览器/小程序本地计时累计值。
- 页面内使用紧凑的可点击过程行：运行时显示摘要与秒数，完成后显示“查看处理过程”与总耗时。

## 任务 1：定义过程数据契约并持久化开始时间

**文件：**
- 修改：`server/internal/domain/agent/model.go`
- 修改：`server/internal/domain/agent/repo.go`
- 修改：`server/internal/domain/agent/repo_test.go`

1. 先为 `CreateChatMessage` 写失败测试，验证创建消息时带有服务端生成/传入的 UTC `created_at`，返回对象包含该时间。
2. 运行 `go test ./internal/domain/agent -run TestRepositoryCreateChatMessage`，确认因字段和 SQL 缺失而失败。
3. 为 `ChatMessage`、创建输入和流完成消息增加开始/完成时间字段；新增只含必要审计字段的 `AgentRunStep`、安全的过程/历史 DTO。
4. 修改消息插入语句显式写入 `created_at`，返回相同时间。
5. 运行上述测试，确认通过。

## 任务 2：查询可恢复的可见消息与安全步骤

**文件：**
- 修改：`server/internal/domain/agent/repo.go`
- 修改：`server/internal/domain/agent/repo_test.go`
- 修改：`server/internal/domain/agent/service.go`
- 修改：`server/internal/domain/agent/service_test.go`

1. 先写失败测试：按时间升序返回用户可见消息；批量读取指定助理消息的审计步骤；服务层把进行中的助理消息映射为带 `created_at` 的运行中过程。
2. 运行相应单测并确认失败原因是缺少查询/服务方法。
3. 增加仓储查询，始终按用户约束；步骤查询仅选取映射需要的字段。
4. 在服务层建立集中式安全映射，将工具操作和步骤类型转换为中文摘要/详情，禁止把审计原文透传。
5. 运行仓储和服务层相关测试。

## 任务 3：在 Agent 执行路径发出安全过程事件

**文件：**
- 修改：`server/internal/domain/agent/service.go`
- 修改：`server/internal/domain/agent/service_test.go`

1. 先写失败测试：调用带过程回调的聊天方法时，首个事件含非零 `response_started_at` 与“理解你的需求”；正常完成后有“完成回复”；结果携带相同开始时间与非零结束时间。
2. 运行该测试，确认 API 尚不存在而失败。
3. 以现有 `Chat` 为兼容包装，新增接收过程回调的聊天方法。
4. 在创建助理占位消息后、模型运行前后、工具执行前后和最终落库后发出安全摘要；失败分支也发出安全终止状态。
5. 运行服务层所有测试。

## 任务 4：提供 SSE 与会话恢复 HTTP 接口

**文件：**
- 修改：`server/internal/domain/agent/handler.go`
- 修改：`server/internal/domain/agent/routes.go`
- 修改：`server/internal/domain/agent/routes_test.go`

1. 先写失败路由测试，验证 `/chat` 输出至少一个 `event: process`，事件内有 `summary` 与 `response_started_at`，`done` 有 `finished_at`；验证 `GET /messages` 只返回可见消息和安全过程字段。
2. 运行测试，确认路由或事件尚不存在。
3. Handler 将服务过程回调编码为 SSE `process` 并立即 flush；`done` 包含开始/结束时间。
4. 注册 `GET /api/user/agent/messages`，调用历史服务方法并返回统一响应。
5. 运行路由测试和 `go test ./internal/domain/agent`。

## 任务 5：小程序恢复计时、步骤展开与完成态

**文件：**
- 修改：`miniapp/utils/api.js`
- 修改：`miniapp/utils/api-client.test.js`
- 修改：`miniapp/pages/advisor/advisor.js`
- 修改：`miniapp/pages/advisor/advisor.wxml`
- 修改：`miniapp/pages/advisor/advisor.wxss`
- 修改：`miniapp/pages/advisor/advisor.test.js`

1. 先为 API 客户端写失败测试，验证恢复接口请求 `GET /api/user/agent/messages`；为页面辅助函数写失败测试，验证从服务端开始时间计算经过秒数，并在完成时选用最后安全摘要。
2. 运行 Node 测试，确认新接口/辅助函数不存在。
3. 添加 API 方法和页面历史恢复；恢复中的助理消息用服务端时间启动前端每秒刷新，不以页面加载时间重置。
4. 处理 SSE `process` 与 `done`：阶段摘要动态更新，完成后显示“查看处理过程”，并把过程详情保存至该消息。
5. 在 WXML 添加位于助理正文上方的紧凑过程控件，支持展开/收起；CSS 实现稳定尺寸、旋转指示和无文本“正在处理”的运行态。
6. 运行小程序 Node 测试。

## 任务 6：端到端验证与人工检查

**文件：**
- 如需修复则仅限以上文件。

1. 运行 `go test ./...` 和小程序现有测试命令。
2. 启动本地服务或现有原型服务，检查 `process` SSE 事件顺序、开始时间格式及完成态字段。
3. 用浏览器检查紧凑处理控件在桌面和窄屏下不重叠，展开后详情可读。
4. 检查 `git diff --check` 与 `git status --short`，确认无临时原型文件被纳入提交。
5. 提交仅本任务文件，提交说明使用中文。
