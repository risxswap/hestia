# Agent Advice Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Agent 聊天草稿建议的第一版服务端闭环，使用 `sections` 分段模型存储草稿和正式建议。

**Architecture:** `agent` domain 负责聊天入口、草稿 tool 能力、草稿确认接口和 SSE 输出。数据库用 `advice_drafts` 作为草稿容器，`advice_draft_sections` 保存当前内容，`advice_draft_section_versions` 保存分段历史，确认后写入 `advices + advice_sections`。本计划先落最小可验证闭环，ADK 事件消费和真实 LLM tool loop 后续在此基础上补充。

**Tech Stack:** Go 1.25、Gin、sqlx、sqlmock、MySQL 初始 schema、项目现有 `response.WriteSSE`。

---

### Task 1: MySQL Schema

**Files:**
- Modify: `server/internal/infra/migration/mysql/001_init_schema.sql`
- Create: `server/internal/infra/migration/mysql/agent_advice_schema_test.go`

- [ ] **Step 1: Write failing schema tests**

Add tests that read `001_init_schema.sql` and assert:

```go
func TestAgentAdviceSectionsSchemaUsesSectionTables(t *testing.T) {
    raw := readInitSchema(t)
    mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_drafts`")
    mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_draft_sections`")
    mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_draft_section_versions`")
    mustContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_sections`")
    mustContain(t, raw, "`current_revision_no` int unsigned NOT NULL DEFAULT 1")
    mustContain(t, raw, "`section_type` varchar(32) NOT NULL")
    mustNotContain(t, raw, "CREATE TABLE IF NOT EXISTS `advice_requests`")
    mustNotContain(t, raw, "`advice_request_id`")
}
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/infra/migration/mysql -run TestAgentAdviceSectionsSchemaUsesSectionTables -count=1
```

Expected: fail because the new section tables do not exist and old `advice_requests` / `advice_request_id` still exist.

- [ ] **Step 3: Update schema**

Remove `advice_requests`. Replace `advices` JSON-body columns with container fields:

```sql
source_msg_id bigint unsigned DEFAULT NULL,
source_draft_id bigint unsigned DEFAULT NULL,
source_draft_revision_no int unsigned DEFAULT NULL,
scene_label varchar(180) DEFAULT NULL,
target_date date DEFAULT NULL,
occasion varchar(180) DEFAULT NULL,
weather_text varchar(255) DEFAULT NULL,
mood_text varchar(255) DEFAULT NULL,
style_goal text,
avoid_goal text
```

Add `advice_drafts`, `advice_draft_sections`, `advice_draft_section_versions`, and `advice_sections` with unique keys from the spec.

- [ ] **Step 4: Verify green**

Run:

```bash
go test ./internal/infra/migration/mysql -count=1
```

Expected: pass.

### Task 2: Agent Domain Model And Repository

**Files:**
- Modify: `server/internal/domain/agent/model.go`
- Modify: `server/internal/domain/agent/repo.go`
- Create: `server/internal/domain/agent/repo_test.go`

- [ ] **Step 1: Write failing repository tests**

Cover these behaviors:

```go
func TestRepositoryCreateDraftCreatesSectionsAndVersions(t *testing.T)
func TestRepositoryUpdateDraftOnlyVersionsChangedSections(t *testing.T)
func TestRepositoryConfirmDraftCreatesAdviceSections(t *testing.T)
```

Use `sqlmock` and assert transaction boundaries, inserts into `advice_drafts`, `advice_draft_sections`, `advice_draft_section_versions`, `advices`, `advice_sections`, and the absence of inserts for unchanged sections.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/domain/agent -run 'TestRepository(CreateDraft|UpdateDraft|ConfirmDraft)' -count=1
```

Expected: fail because repository methods and models do not exist.

- [ ] **Step 3: Implement model and repository**

Add types:

```go
type Draft struct { ID int64; PublicID string; UserID int64; Status string; CurrentRevisionNo int; Sections []DraftSection }
type DraftSection struct { ID int64; PublicID string; DraftID int64; SectionType string; CurrentSectionVersionID int64; CurrentSectionVersionNo int; ContentSchemaVersion string; ContentJSON map[string]any }
type DraftSectionVersion struct { ID int64; PublicID string; DraftID int64; SectionID int64; SectionType string; SectionVersionNo int; DraftRevisionNo int; ContentJSON map[string]any }
type Advice struct { ID int64; PublicID string; UserID int64; Status string; SourceDraftID int64; SourceDraftRevisionNo int; Sections []AdviceSection }
```

Repository methods:

```go
CreateDraft(ctx context.Context, input CreateDraftInput) (Draft, error)
GetCurrentDraft(ctx context.Context, userID int64) (Draft, error)
UpdateDraftSections(ctx context.Context, input UpdateDraftInput) (Draft, error)
DiscardDraft(ctx context.Context, userID int64, publicID string) error
ConfirmDraft(ctx context.Context, userID int64, publicID string) (Advice, error)
```

- [ ] **Step 4: Verify green**

Run:

```bash
go test ./internal/domain/agent -count=1
```

Expected: pass.

### Task 3: Agent Chat And Draft HTTP APIs

**Files:**
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/agent/handler.go`
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/routes_test.go`

- [ ] **Step 1: Write failing route tests**

Cover:

```go
func TestChatRouteUsesAgentChatPathAndReturnsDraftEvent(t *testing.T)
func TestConfirmDraftRouteReturnsAdvicePublicID(t *testing.T)
func TestDiscardDraftRouteMarksDraftDiscarded(t *testing.T)
func TestCurrentDraftRouteReturnsDraftCard(t *testing.T)
```

Assert `/api/user/agent/chat`, `/api/user/advice-drafts/current`, `/api/user/advice-drafts/:public_id/confirm`, and `/api/user/advice-drafts/:public_id/discard`.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/domain/agent -run 'Test(ChatRoute|ConfirmDraftRoute|DiscardDraftRoute|CurrentDraftRoute)' -count=1
```

Expected: fail because routes do not exist.

- [ ] **Step 3: Implement minimal service and handlers**

`POST /api/user/agent/chat` writes SSE events `status`, `message`, optional `draft`, `done`. First version uses deterministic local draft creation/update logic when no real ADK runner is configured. Save is still only through confirm button.

Register non-Agent draft routes under the user API group from `cmd/server/main.go` or the existing route registration point.

- [ ] **Step 4: Verify green**

Run:

```bash
go test ./internal/domain/agent -count=1
```

Expected: pass.

### Task 4: Integration Verification

**Files:**
- Modify as needed based on compiler errors only.

- [ ] **Step 1: Run focused tests**

```bash
go test ./internal/domain/agent ./internal/infra/migration/mysql -count=1
```

- [ ] **Step 2: Run server tests**

```bash
go test ./...
```

Expected today: may still fail in `internal/domain/onboarding` because baseline currently fails with `memoryProfileRepo` missing `CreateProfilePhoto`. Do not hide that; report it clearly.

- [ ] **Step 3: Commit**

Commit schema and server implementation in focused commits.
