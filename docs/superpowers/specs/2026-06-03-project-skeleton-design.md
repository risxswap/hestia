# 项目骨架设计

日期：2026-06-03

## 目标

建立第一版可继续开发的项目骨架，覆盖微信小程序、用户 Web、管理端和 Go 双服务入口。骨架只放必要启动、路由、页面占位和验证文件，不实现完整业务流程。

## 技术选型

- 小程序：微信原生小程序 + Lin UI。
- 用户 Web：Vue 3 + Vite + Naive UI。
- 管理端：Vue 3 + Vite + Element Plus。
- 包管理：三个前端项目各自独立维护 `package.json` 和 `package-lock.json`，不使用根目录 npm workspaces。
- 服务端：`server` 下保留 `user-server` 和 `admin-server` 两个 Go 入口。

## 目录结构

```text
miniapp/
user/
admin/
server/
  cmd/
    user-server/
    admin-server/
  internal/app/
    user/
    admin/
```

## 小程序骨架

`miniapp` 作为微信开发者工具项目根目录，保留 `app.json`、`project.config.json`、`lin-ui.config.json` 和核心页面：

- 顾问。
- 报告。
- 衣橱。
- 我的。
- Onboarding。

Lin UI 使用 npm 依赖，组件引用指向微信开发者工具构建后的 `miniprogram_npm/lin-ui`。该目录是构建产物，不提交。

## 用户 Web 骨架

`user` 使用 Vue Router 组织：

- `/` 首页工作台。
- `/report` 报告。
- `/recommendations` 历史建议。
- `/settings` 设置。
- `/share/:type/:token` 分享页。

视觉沿用温和专业方向，适合报告复盘和轻反馈。

## 管理端骨架

`admin` 使用 Element Plus 的后台布局，一级导航包括：

- 工作台。
- 风格库。
- AI 配置。
- 任务。
- 用户。
- 系统。

第一版保持运营工具风格，优先支持后续接入表格、筛选、详情和状态流。

## 验证

骨架完成后运行：

- `cd user && npm install && npm run build`
- `cd admin && npm install && npm run build`
- `cd miniapp && npm install`
- `cd server && go test ./...`
