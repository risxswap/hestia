# Hestia 服务端架构设计

日期：2026-06-20

## 背景

Hestia 是个人 AI 形象顾问 Agent，第一版以微信小程序为主入口，用户 Web 用于报告复盘、历史建议和分享访问，管理端用于风格库、AI 配置、任务和用户运营支持。

当前仓库已有 `server` 骨架，包含 `user-server` 和 `admin-server` 两个 Go 入口。后续服务端使用 Golang、Gin 和 Eino，并采用轻量 MVC 架构。本文档定义第一版服务端架构，不调整既有产品边界和数据库逻辑设计。

本设计沿用已经确认的产品闭环：

```text
轻量 onboarding
→ 初版行动型形象报告
→ 每日场景穿搭/发型建议
→ 用户反馈
→ 记忆更新
→ 下一次建议变准
```

## 架构原则

第一版服务端采用：

```text
轻量 MVC + 领域模块
```

这里的 `domain` 只表示业务模块集合，不引入重型 DDD。项目不使用聚合根、领域事件、仓储接口等复杂战术建模。每个业务模块内部使用 Go 文件后缀区分职责，而不是再拆 `controller`、`repository` 等子包。

核心原则：

- 使用 Gin 作为 HTTP 框架。
- 使用 `database/sql + sqlx` 访问 MySQL。
- 使用 Go 生态迁移工具管理数据库迁移。
- 使用 Eino 实现智能体编排。
- `agent` 是一等业务模块，对外暴露智能交互 API。
- 每个业务模块按对外服务面暴露用户侧或管理侧路由，由 `app/user` 和 `app/admin` 统一装配。
- 不做 DDD，不做微服务拆分，不做与 MVP 无关的复杂抽象。
- 关键用户信息必须结构化落库，不能只依赖聊天上下文。
- 用户自拍、衣橱、偏好、反馈和记忆都按敏感数据处理。

## 总体目录

推荐目录：

```text
server/
  cmd/
    user-server/
    admin-server/
  migrations/
  internal/
    app/
      user/
      admin/
    domain/
      agent/
      account/
      profile/
      asset/
      wardrobe/
      stylelib/
      report/
      advice/
      feedback/
      memory/
      job/
    infra/
      config/
      mysql/
      redis/
      queue/
      oss/
      wechat/
      llm/
      logger/
    common/
      response/
      middleware/
      errors/
```

### 服务入口

`user-server` 服务：

- 微信小程序 API。
- 用户 Web API。
- Agent SSE 智能交互。
- 第一版同进程启动 Asynq worker。

`admin-server` 服务：

- 管理端 API。
- 风格库维护。
- AI 配置。
- 用户运营支持。
- 任务查看和重试。

`admin-server` 第一版不消费用户侧任务。

### 业务模块结构

每个业务模块内部使用文件后缀表达职责：

```text
domain/advice/
  routes.go
  handler.go
  service.go
  model.go
  repo.go
```

职责：

- `routes.go`：注册 Gin 路由。
- `handler.go`：处理 HTTP 入参、鉴权上下文、响应映射。
- `service.go`：业务流程、事务、跨模块编排。
- `model.go`：数据库模型、请求响应结构、模块内值对象。
- `repo.go`：SQL 查询和持久化。

每个模块按实际需要暴露用户侧或管理侧路由：

```go
func RegisterUserRoutes(group *gin.RouterGroup, deps *Deps)
func RegisterAdminRoutes(group *gin.RouterGroup, deps *Deps)
```

不是每个模块都必须同时实现两类路由。`agent`、`profile`、`wardrobe`、`report`、`advice`、`feedback`、`memory` 主要面向用户侧；`stylelib`、`job`、运营配置主要面向管理侧。`app/user` 和 `app/admin` 根据服务边界选择注册哪些模块。

## Agent 模块

`agent` 是智能体业务模块，不是隐藏在基础设施里的工具。用户侧的主要智能交互统一进入 `agent`，其中小程序是第一版主体验入口。

`agent` 覆盖：

- 聊聊。
- 今日建议。
- Onboarding。
- 拍照问搭配。
- 参考风格解析。
- 需要智能推理的反馈追问。

推荐结构：

```text
domain/agent/
  routes.go
  handler.go
  service.go
  model.go
  repo.go
  intent.go
  context.go
  planner.go
  eino.go
  tools.go
  prompt.go
```

职责：

- 接收用户消息、图片引用、场景上下文。
- 写入 `chat_msgs`。
- 判断意图和任务类型。
- 读取 `profile`、`wardrobe`、`memory`、`stylelib` 等上下文。
- 使用 Eino 完成智能体推理和工具调用。
- 调用 `report`、`advice`、`feedback`、`memory` 等模块 service 写入结构化结果。
- 通过 SSE 推送状态、文本片段、业务卡片和结束事件。

依赖规则：

```text
agent 可以调用其他领域模块 service。
其他领域模块不反向调用 agent。
```

这样可以避免 Go 包循环依赖，也能让 Agent 保持产品主入口心智。

## HTTP API 边界

用户侧 API：

```text
/api/user/*       微信小程序和用户 Web
```

管理端 API：

```text
/api/admin/*
```

小程序和用户 Web 都属于终端用户侧，权限模型相同，都使用终端用户 session。两端差异主要来自客户端能力和展示 DTO，例如小程序承载完整 Agent 交互，用户 Web 更偏报告复盘、历史建议、轻反馈和分享访问；这些差异不通过额外的服务端权限面拆分。

### Agent SSE 主入口

用户侧智能交互主入口：

```text
POST /api/user/agent/stream
Accept: text/event-stream
```

服务端返回 SSE 事件：

```text
event: status
data: {"text":"我在整理你的场景和衣橱"}

event: token
data: {"text":"今天可以优先选择..."}

event: card
data: {"type":"advice","public_id":"adv_xxx"}

event: done
data: {"job_public_id":"job_xxx","message_public_id":"msg_xxx"}
```

错误事件：

```text
event: error
data: {"code":"agent.stream_failed","message":"生成建议失败，请稍后重试"}
```

SSE 只负责实时体验。最终状态仍然落到：

- `chat_msgs`
- `jobs`
- `reports`
- `advices`
- `feedbacks`
- `memories`

连接中断后，前端可以使用 `job_public_id` 查询最终状态。

微信小程序侧不依赖浏览器 `EventSource`。小程序需要使用分块接收方式封装 SSE 解析，并处理半包、取消、超时、重连和 UTF-8 解码。

### 确定性业务 API

普通查询、反馈和修正不走 Agent 主入口：

```text
GET    /api/user/reports/:id
GET    /api/user/advices/:id
POST   /api/user/advices/:id/feedback
GET    /api/user/memories
DELETE /api/user/memories/:id
PATCH  /api/user/profile/facts/:id
PATCH  /api/user/profile/inferences/:id
GET    /api/user/wardrobe/items
POST   /api/user/wardrobe/items
```

这些 API 用于报告查看、建议查看、反馈提交、记忆删除、画像修正和核心衣橱管理。

## 异步任务

第一版使用 `hibiken/asynq` 作为 Go 生态成熟任务队列。Asynq 基于 Redis，负责队列、worker 并发、重试、延迟任务和失败队列。

MySQL `jobs` 表负责业务状态和审计。Asynq 是执行层，`jobs` 是产品业务层。

目录：

```text
infra/queue/
  asynq.go

domain/job/
  routes.go
  handler.go
  service.go
  model.go
  repo.go
  tasks.go
  worker.go
```

`jobs` 记录：

- `job_public_id`
- `user_id`
- `task_type`
- `status`
- `target_type`
- `target_id`
- `input_summary`
- `output_summary`
- `error_message`
- `retry_count`
- `started_at`
- `finished_at`

第一版 worker 随 `user-server` 同进程启动：

```text
cmd/user-server
→ 初始化 Config / DB / Redis / Asynq / Deps
→ 启动 Gin HTTP 服务
→ 启动 Asynq worker
```

后续如果任务压力变大，可以新增 `cmd/worker`，复用 `job.NewWorker(deps)`。

管理端只查看和触发重试：

```text
GET  /api/admin/jobs
GET  /api/admin/jobs/:id
POST /api/admin/jobs/:id/retry
```

第一版任务类型：

```text
agent.message.process
asset.image.extract
onboarding.extract
report.generate
advice.generate
memory.summarize
asset.cleanup
```

SSE 与 job 的关系：

- 轻量 Agent 交互可以在当前请求中直接流式返回，并写入 `chat_msgs`。
- 重任务必须先创建 `jobs` 记录，再投递 Asynq。
- SSE 连接可推送任务的阶段状态、文本片段和业务卡片。
- SSE 连接不是任务状态的唯一来源；任务完成、失败和重试状态以 `jobs` 表为准。
- 前端断线后，可以用 `job_public_id` 恢复任务状态和最终业务对象。

## 数据访问与事务

数据访问使用：

```text
database/sql + sqlx
```

不使用 GORM。

基础设施：

```text
infra/mysql/
  mysql.go
```

模块 repo 示例：

```go
type Repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}
```

事务由 `service.go` 负责开启和提交。repo 方法可以接收 `sqlx.ExtContext`，让同一个方法同时支持 `db` 和 `tx`：

```go
func (r *Repo) Create(ctx context.Context, exec sqlx.ExtContext, item *Advice) error
```

事务原则：

- 单表简单写入可由 repo 直接执行。
- 跨表写入必须由 service 开事务。
- Agent 编排不直接跨表裸写，必须通过各模块 service。
- 隐私删除、反馈生成记忆、建议生成结果落库，必须显式事务。
- 不依赖数据库外键级联删除，由应用层编排软删除和资源清理。

## 数据库迁移

数据库迁移使用 Go 生态迁移工具：

```text
golang-migrate/migrate
```

目录：

```text
server/
  migrations/
    000001_create_users.up.sql
    000001_create_users.down.sql
    000002_create_assets.up.sql
    000002_create_assets.down.sql
```

迁移执行入口集成到 `admin-server`，不单独保留 `cmd/migrate`。`admin-server` 作为管理侧进程，负责承载运维管理类能力；迁移命令以 CLI 子命令形式提供，不暴露为普通 HTTP API。

本地命令：

```bash
cd server
go run ./cmd/admin-server migrate up
go run ./cmd/admin-server migrate down 1
go run ./cmd/admin-server migrate version
```

迁移命令读取和 `admin-server` 一致的配置，例如 `DATABASE_DSN`。线上环境默认不自动执行迁移，避免服务启动时隐式修改表结构；需要由部署流程或人工运维显式执行 `admin-server migrate` 子命令。

## 配置与依赖装配

配置策略：

- 环境变量为准。
- 开发环境支持 `server/.env`。
- 缺必填配置启动失败。
- 不在代码里写真实密钥。

配置目录：

```text
infra/config/
  config.go
```

示例：

```go
type Config struct {
	AppEnv      string `env:"APP_ENV" envDefault:"development"`
	UserPort    string `env:"USER_SERVER_PORT" envDefault:"8080"`
	AdminPort   string `env:"ADMIN_SERVER_PORT" envDefault:"8081"`
	DatabaseDSN string `env:"DATABASE_DSN,required"`
	RedisAddr   string `env:"REDIS_ADDR,required"`
	LLMProvider string `env:"LLM_PROVIDER"`
}
```

依赖装配使用手写 `Deps`，不用 Wire、Fx 等 DI 工具：

```go
type Deps struct {
	Config *config.Config
	DB     *sqlx.DB
	Redis  *redis.Client
	Queue  *asynq.Client
	LLM    llm.Client
	Logger *slog.Logger
}
```

`app/user` 和 `app/admin` 负责初始化 Gin、挂载中间件、注册模块路由。

## 鉴权与会话

小程序第一版使用微信登录：

- 前端传微信 `code`。
- 服务端用 `code` 换 openid/unionid。
- 创建或读取 `users`。
- 返回 Redis session token。

用户 Web 第一版支持安全访问 token 或分享 token。手机号登录后续扩展。

管理端使用独立管理员账号：

- username + password。
- 独立 Redis session token。

session 存 Redis，不使用无状态 JWT：

```text
sess:user:<token>
sess:admin:<token>
```

Redis session token 能支持即时注销、封禁、删除账号后立即失效，更适合 Hestia 的隐私敏感场景。

MySQL 不存登录 session，只存账号、管理员账号和必要审计信息。

## 对象存储

对象存储继续使用七牛云。

业务模块：

```text
domain/asset
```

基础设施：

```text
infra/oss
```

上传链路：

```text
1. 前端请求上传凭证。
2. 服务端校验用户、用途、文件类型和大小限制。
3. 服务端返回七牛 token、key、bucket、expire。
4. 前端直传七牛云。
5. 前端回传上传结果。
6. 服务端创建 assets 记录。
7. 后台任务做图片识别、审核或资源清理。
```

隐私原则：

- 自拍、半身照、衣橱照、参考图都进入 `assets`。
- 用户上传资产默认 private。
- 删除用户数据时，必须删除或标记清理七牛云资源。
- 未经授权，不公开复用用户上传图片。

## LLM 与 Eino 边界

`infra/llm` 只封装底层模型供应商调用：

- 请求发送。
- 超时。
- 重试。
- 模型配置。
- 结构化输出基础能力。

`infra/llm` 不写业务 prompt，不知道 Hestia 的画像、衣橱、建议、记忆语义。

Eino 属于 `domain/agent`：

- 智能体编排。
- 工具调用。
- prompt 组合。
- 上下文选择。
- 输出解析。

这样能让 Agent 成为明确的业务模块，同时保持模型供应商适配可替换。

## 日志与响应

日志使用 Go 标准库：

```text
log/slog
```

基础设施：

```text
infra/logger/
  logger.go
```

请求日志由 Gin middleware 注入：

- `request_id`
- `user_id`
- `admin_user_id`
- `path`
- `method`
- `status`
- `latency_ms`
- `job_public_id`
- `agent_trace_id`

AI 任务输入输出摘要写入业务表，不只依赖日志。

API 统一响应：

```json
{
  "code": "ok",
  "message": "",
  "data": {}
}
```

错误响应：

```json
{
  "code": "profile.not_found",
  "message": "画像不存在",
  "data": null
}
```

错误码使用字符串，格式：

```text
<module>.<reason>
```

示例：

```text
account.unauthorized
asset.invalid_usage
agent.stream_failed
advice.not_found
memory.delete_denied
job.retry_failed
```

公共能力：

```text
common/
  response/
    response.go
    sse.go
  errors/
    errors.go
  middleware/
    auth.go
    request_id.go
    recovery.go
```

## 测试与验证

测试策略：

- handler 测试：验证路由、鉴权、响应格式。
- service 测试：验证事务、跨模块编排和错误分支。
- repo 测试：覆盖关键 SQL 和查询条件。
- agent 测试：用 fake llm、fake tool 验证意图分流和结构化输出。
- worker 测试：验证 Asynq task handler 更新 jobs 状态。
- SSE 测试：验证事件顺序、错误事件和 done 事件。

第一版最小验证命令：

```bash
cd server
go test ./...
go run ./cmd/admin-server migrate version
```

在迁移命令实现前，至少运行：

```bash
cd server
go test ./...
```

## MVP 验收标准

服务端架构满足第一版开发条件，当且仅当：

- `user-server` 和 `admin-server` 两个入口清楚。
- `user-server` 支持小程序、用户 Web 和 Agent SSE。
- `admin-server` 只服务管理端。
- 业务模块采用轻量 MVC 文件结构。
- 各模块按服务面暴露 `RegisterUserRoutes` 或 `RegisterAdminRoutes`，由 app 统一装配。
- `agent` 是一等业务模块，并作为智能交互主入口。
- 其他领域模块不反向调用 `agent`。
- SSE 用于 Agent 实时体验，最终结果结构化落库。
- 异步任务使用 Asynq，`jobs` 表保留业务状态。
- 第一版 worker 随 `user-server` 启动，并预留后续独立 worker 边界。
- MySQL 使用 `database/sql + sqlx`。
- 数据库迁移使用 Go 生态迁移工具。
- session 使用 Redis token。
- 七牛云资产删除和隐私删除被纳入服务端设计。
- API 响应和错误码格式统一。
- 测试覆盖 handler、service、repo、agent、worker 和 SSE 的核心行为。
