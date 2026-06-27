# 衣橱单品弹窗编辑 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将核心衣橱页面的新增/编辑表单从页内卡片改为底部弹窗操作。

**Architecture:** 保留 `wardrobe.js` 的现有数据模型和保存逻辑，只增加关闭弹窗的 handler。`wardrobe.wxml` 将 `editorVisible` 对应节点改为固定层弹窗，`wardrobe.wxss` 提供遮罩、抽屉、滚动内容和固定操作区样式。

**Tech Stack:** 微信小程序 WXML/WXSS/JavaScript，现有 Node 验证脚本。

---

### Task 1: 弹窗行为验证

**Files:**
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写失败验证**

在衣橱页断言中加入：

```js
assert(typeof wardrobe.config.handleCloseEditor === "function", "wardrobe should close item editor modal");
```

在 `handleOpenCreate` 后加入：

```js
assert(wardrobeInstance.data.editorVisible === true, "wardrobe create should open editor modal");
wardrobe.config.handleCloseEditor.call(wardrobeInstance);
assert(wardrobeInstance.data.editorVisible === false, "wardrobe close should hide editor modal");
assert(wardrobeInstance.data.editingPublicID === "", "wardrobe close should clear editingPublicID");
assert(wardrobeInstance.data.draft.name === "", "wardrobe close should reset draft");
wardrobe.config.handleOpenCreate.call(wardrobeInstance);
```

在 WXML 断言中加入：

```js
assert(wardrobeMarkup.includes("modal-layer"), "wardrobe editor should render as modal layer");
assert(wardrobeMarkup.includes("modal-backdrop"), "wardrobe editor should include a backdrop");
assert(wardrobeMarkup.includes("modal-sheet"), "wardrobe editor should use a bottom sheet");
assert(wardrobeMarkup.includes("handleCloseEditor"), "wardrobe page should bind close editor action");
assert(!wardrobeMarkup.includes("class=\"card editor-panel\""), "wardrobe editor should not render as inline card");
```

- [ ] **Step 2: 运行验证确认失败**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL，提示缺少 `handleCloseEditor` 或弹窗结构。

### Task 2: 实现弹窗结构

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`

- [ ] **Step 1: 增加关闭 handler**

在 `wardrobePageConfig` 中增加：

```js
handleCloseEditor() {
  this.setData({
    editorVisible: false,
    editingPublicID: "",
    draft: cloneDraft(),
    errorMessage: ""
  });
},
```

- [ ] **Step 2: 把页内表单改为底部弹窗**

将 `editorVisible` 节点替换为 `modal-layer`、`modal-backdrop`、`modal-sheet`、`modal-scroll` 和 `modal-actions` 结构，保留原表单字段与事件绑定。

- [ ] **Step 3: 添加弹窗样式**

新增固定遮罩、底部抽屉、可滚动内容、操作区样式；复用现有字段、按钮和状态样式。

### Task 3: 验证与收尾

**Files:**
- Verify only

- [ ] **Step 1: 运行小程序集成验证**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS。

- [ ] **Step 2: 运行 API 客户端验证**

Run: `cd miniapp && npm run verify:api-client`

Expected: PASS。

- [ ] **Step 3: 检查工作区**

Run: `git status --short`

Expected: 只包含本次设计、计划、衣橱页和验证脚本相关文件。
