# 项目骨架实现计划

日期：2026-06-03

## 步骤

1. 创建根目录 `user`，接入 Vue、Vite、Naive UI 和基础路由。
2. 创建根目录 `admin`，接入 Vue、Vite、Element Plus 和基础路由。
3. 创建根目录 `miniapp`，接入微信原生小程序结构和 TDesign MiniProgram npm 工作流。
4. 让三个前端项目各自维护依赖和锁文件，不使用根目录 npm workspaces。
5. 创建 `server` 双 Go 入口和 health 路由测试。
6. 分别安装依赖并运行构建、类型检查和服务端测试。
