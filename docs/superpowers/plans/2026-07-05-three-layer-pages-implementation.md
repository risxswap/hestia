# 私藏与我的页三层架构实施计划

> 对应 spec：`docs/superpowers/specs/2026-07-05-collection-three-layer-pages-design.md`

## 目标

一次性把当前“私藏/我的”页面和后端模型切到新结构：

- 页面路由采用 `pages/<domain>/<list|detail|edit|index>` 风格。
- 衣服域统一从 `wardrobe` 迁移到 `clothes`。
- 发型、妆容补齐独立表、CRUD API、列表/详情/编辑闭环。
- `我的` 页拆成档案、偏好与禁忌、记忆、隐私与数据等独立域。
- 详情页严格只读，编辑页承载表单，列表页只保留轻量新增弹层。

## 约束与风险

- 当前 MySQL migration 没有 `schema_migrations` 版本表，所有 SQL 会按文件名重复执行；不能直接写非幂等 `RENAME TABLE`。
- 用户已明确仍在开发阶段，不考虑旧页面/旧 API 兼容，因此小程序入口、验证脚本、API 客户端一次性改到新命名。
- 主工作区已有未提交改动，本次实现只在隔离 worktree `/Users/ming/.config/superpowers/worktrees/hestia/codex-implement-three-layer-pages` 中进行。
- 新增 hair/makeup 表不使用 JSON 承载标签，适用场景用独立标签表。
- `references` 暂未想清楚，前端入口和集合聚合先移除；后端旧占位路由可以暂时不注册到用户路由。

## 实施步骤

### 1. 基线验证

已完成：

```bash
cd /Users/ming/.config/superpowers/worktrees/hestia/codex-implement-three-layer-pages/server
/usr/local/go/bin/go test ./...

cd /Users/ming/.config/superpowers/worktrees/hestia/codex-implement-three-layer-pages/miniapp
/usr/local/bin/npm run verify:api-client
/usr/local/bin/npm run verify:api-integration
/usr/local/bin/npm run verify:profile-page
/usr/local/bin/npm run verify:private-tab-naming
```

期望：全部通过，作为改动前基线。

### 2. 后端 clothes 迁移

文件范围：

- `server/internal/domain/clothes/**`
- `server/internal/domain/wardrobe/**` 删除或停止注册后再清理
- `server/internal/domain/collection/**`
- `server/internal/app/user/router.go`
- `server/internal/infra/migration/mysql/*.sql`
- `server/internal/infra/migration/corrections.go`
- 相关测试文件

实现要点：

- 从现有 `wardrobe` 领域迁移出 `clothes` 领域，保留识别能力和现有业务行为。
- API 改为：
  - `GET /api/user/clothes/options`
  - `GET /api/user/clothes/items`
  - `GET /api/user/clothes/items/:public_id`
  - `POST /api/user/clothes/items`
  - `POST /api/user/clothes/items/recognize`
  - `PATCH /api/user/clothes/items/:public_id`
  - `DELETE /api/user/clothes/items/:public_id`
- 表名改为：
  - `clothes`
  - `clothes_assets`
  - `clothes_gaps`
- 配置 group 改为 `clothes.item_options`。
- `files.asset_type` 和 object key 路径改为 `clothes_item_photo` / `users/{id}/clothes/...`。
- migration 要保持可重复执行：优先更新初始化 schema 到新表名；对已有开发库的数据迁移放进 Go correction，通过 `information_schema` 判断旧表存在再迁移，避免重复运行失败。

测试先行：

- 新增/迁移 `clothes` route/service/repo 测试，先让旧路径断言失败，再实现。
- collection 测试先断言 `type=clothes`、入口为 `/pages/clothes/list`、recent item 指向 `/pages/clothes/detail`。

### 3. 后端 hair/makeup CRUD

文件范围：

- `server/internal/domain/hair/**`
- `server/internal/domain/makeup/**`
- `server/internal/domain/collection/**`
- `server/internal/infra/migration/mysql/*.sql`
- `server/internal/infra/migration/corrections.go`

实现要点：

- hair API：
  - `GET /api/user/hair`
  - `GET /api/user/hair/:public_id`
  - `POST /api/user/hair`
  - `PATCH /api/user/hair/:public_id`
  - `DELETE /api/user/hair/:public_id`
- makeup API：
  - `GET /api/user/makeup`
  - `GET /api/user/makeup/:public_id`
  - `POST /api/user/makeup`
  - `PATCH /api/user/makeup/:public_id`
  - `DELETE /api/user/makeup/:public_id`
- 新增表：
  - `hair`
  - `hair_assets`
  - `hair_scene_tags`
  - `makeup`
  - `makeup_assets`
  - `makeup_scene_tags`
- service 做字段 trim、推荐状态校验、软删除、详情权限校验。
- repo 校验资产归属当前用户，跨用户资产不可关联。
- collection 注入 hair/makeup lister，计数和 recent items 来自三类真实数据。

测试先行：

- hair/makeup route 测试覆盖 list/get/create/update/delete、非法推荐状态、未找到。
- repo 测试覆盖标签表写入与替换，确认不依赖 JSON。
- collection 测试覆盖三类计数和 recent item 排序。

### 4. 小程序 API 与路由迁移

文件范围：

- `miniapp/app.json`
- `miniapp/utils/api.js`
- `miniapp/pages/collection/**`
- `miniapp/pages/clothes/**`
- `miniapp/pages/hair/**`
- `miniapp/pages/makeup/**`
- `miniapp/pages/profile/**`
- `miniapp/pages/preferences/**`
- `miniapp/pages/memory/**`
- `miniapp/pages/privacy/**`
- `miniapp/scripts/*.js`

实现要点：

- 删除旧页面入口：
  - `pages/wardrobe/wardrobe`
  - `pages/wardrobe-detail/wardrobe-detail`
  - `pages/wardrobe-edit/wardrobe-edit`
  - `pages/references/references`
- 新增页面入口：
  - `pages/clothes/list`
  - `pages/clothes/detail`
  - `pages/clothes/edit`
  - `pages/hair/list`
  - `pages/hair/detail`
  - `pages/hair/edit`
  - `pages/makeup/list`
  - `pages/makeup/detail`
  - `pages/makeup/edit`
  - `pages/profile/index`
  - `pages/profile/edit`
  - `pages/preferences/edit`
  - `pages/memory/index`
  - `pages/privacy/index`
- tabBar 我的页改为 `pages/profile/index`。
- API 客户端补齐 clothes/hair/makeup CRUD，移除 references 客户端暴露。
- 详情页不出现输入组件、上传组件或保存按钮；编辑页才有表单。
- 衣服、发型、妆容列表页保留轻量新增弹层。

测试先行：

- 更新验证脚本先断言新页面路径和 API 名称，旧 `wardrobe`/`references` 入口不再出现。
- 页面脚本校验轻量新增 payload、详情跳转、编辑保存 payload。

### 5. 验证

最终必须运行：

```bash
cd /Users/ming/.config/superpowers/worktrees/hestia/codex-implement-three-layer-pages/server
/usr/local/go/bin/go test ./...

cd /Users/ming/.config/superpowers/worktrees/hestia/codex-implement-three-layer-pages/miniapp
/usr/local/bin/npm run verify:api-client
/usr/local/bin/npm run verify:api-integration
/usr/local/bin/npm run verify:profile-page
/usr/local/bin/npm run verify:private-tab-naming
```

如时间允许，再启动本地服务并用小程序浏览器检查关键路径：

- 私藏页进入衣服/发型/妆容列表。
- 三类列表轻量新增。
- 三类详情页只读。
- 三类编辑页保存。
- 我的页进入档案、偏好与禁忌、记忆、隐私独立页面。
