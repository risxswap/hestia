# hestia

个人 AI 形象顾问 Agent。第一版以微信小程序为主入口，用户 Web 端用于报告复盘和分享，管理端用于风格库、AI 配置、任务和用户运营支持。

## 项目结构

```text
miniapp/     微信小程序，原生小程序 + TDesign MiniProgram
user/        用户 Web 端，Vue + Naive UI
admin/       管理端，Vue + Element Plus
server/      Go 双服务入口骨架
```

## 本地开发

三个前端项目互相独立，各自在自己的目录安装依赖。

启动用户 Web：

```bash
cd user
npm install
npm run dev
```

启动管理端：

```bash
cd admin
npm install
npm run dev
```

安装小程序依赖：

```bash
cd miniapp
npm install
```

分别运行 Web 前端构建：

```bash
cd user && npm run build
cd admin && npm run build
```

运行服务端测试：

```bash
cd server && go test ./...
```

## 小程序

小程序位于 `miniapp`，使用 TDesign MiniProgram。首次在微信开发者工具中打开前，需要在小程序目录安装依赖并执行“工具 -> 构建 npm”：

```bash
cd miniapp
npm install
```

`miniprogram_npm/` 是微信开发者工具生成产物，不提交到仓库。当前聊聊页直接使用 TDesign 的 `chat-list` 和 `chat-sender` 官方 AI Chat 组件。

TDesign MiniProgram 1.15 的组件运行依赖 `tslib`，小程序目录已在 `dependencies` 中显式声明。若微信开发者工具报 `miniprogram_npm/tdesign-miniprogram/... module is not defined`，先删除本地生成的 `miniapp/miniprogram_npm/`，再在微信开发者工具中重新执行“工具 -> 构建 npm”。`app.json` 不要配置 `"style": "v2"`，否则会影响 TDesign 组件样式。
