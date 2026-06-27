# 图库式衣橱主界面 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将小程序衣橱页升级为图库式主界面，并新增衣服详情页；新增和编辑继续使用底部弹窗。

**Architecture:** 把衣橱纯函数从 `pages/wardrobe/wardrobe.js` 抽到 `miniapp/utils/wardrobe.js`，主界面和详情页共用同一套分类、格式化、payload、草稿转换逻辑。主界面负责全部/分类图库浏览和新增入口，详情页通过现有列表接口按 `public_id` 查找单品并承载查看、编辑、删除流程。

**Tech Stack:** 微信小程序 Page/WXML/WXSS，现有 `miniapp/utils/api.js`，Node 验证脚本。

---

### Task 1: 共享衣橱工具与失败验证

**Files:**
- Create: `miniapp/utils/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写失败验证**

扩展 `miniapp/scripts/verify-miniapp-api-integration.js`：

```js
assert(typeof wardrobe.config.handleOpenDetail === "function", "wardrobe should open item detail page");
assert(
  wardrobe.config.data.categoryOptions.map((item) => item.value).join(",") === "all,top,bottom,outerwear,shoes,bag,accessory,sport,home,other",
  "wardrobe category options should support gallery categories"
);
assert(
  wardrobeInstance.data.categoryOptions[0].countLabel === "全部 2",
  "wardrobe should show all count in category chip"
);
```

并模拟 `wx.navigateTo` 验证：

```js
const originalWx = global.wx;
let wardrobeDetailUrl = "";
global.wx = {
  navigateTo(options) {
    wardrobeDetailUrl = options && options.url ? options.url : "";
  }
};
wardrobe.config.handleOpenDetail.call(wardrobeInstance, {
  currentTarget: {
    dataset: {
      publicId: "wdi_shirt"
    }
  }
});
global.wx = originalWx;
assert(
  wardrobeDetailUrl === "/pages/wardrobe-detail/wardrobe-detail?public_id=wdi_shirt",
  "wardrobe item tap should navigate to detail page"
);
```

- [ ] **Step 2: 运行验证确认失败**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL because `handleOpenDetail` and gallery category counts do not exist yet.

- [ ] **Step 3: 抽共享工具**

Create `miniapp/utils/wardrobe.js` with exports:

```js
module.exports = {
  categoryOptions,
  categoryLabels,
  recommendationLabels,
  cloneDraft,
  normalizeSceneTags,
  normalizeWardrobeItems,
  decorateWardrobeItem,
  filterItems,
  priorityItems,
  normalizeWardrobeGaps,
  buildPayload,
  itemToDraft,
  categoryOptionsWithCounts,
  styleLogicForItem
};
```

`categoryOptions` includes `all, top, bottom, outerwear, shoes, bag, accessory, sport, home, other` and `categoryOptionsWithCounts(items)` returns options with `count` and `countLabel`.

- [ ] **Step 4: 更新主页面引用共享工具**

`miniapp/pages/wardrobe/wardrobe.js` imports `../../utils/wardrobe` and re-exports helpers currently used by tests. `nextWardrobeState(items, category)` returns:

```js
{
  items,
  visibleItems: wardrobeUtils.filterItems(items, category),
  priorityItems: wardrobeUtils.priorityItems(items),
  categoryOptions: wardrobeUtils.categoryOptionsWithCounts(items)
}
```

### Task 2: 图库式衣橱主界面

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写图库结构断言**

Add WXML assertions:

```js
assert(wardrobeMarkup.includes("我的衣服"), "wardrobe gallery should use gallery title");
assert(wardrobeMarkup.includes("category-scroll"), "wardrobe category row should be horizontally scrollable");
assert(wardrobeMarkup.includes("gallery-grid"), "wardrobe page should render gallery grid");
assert(wardrobeMarkup.includes("gallery-add-card"), "wardrobe page should include grid add card");
assert(wardrobeMarkup.includes("handleOpenDetail"), "wardrobe cards should bind detail navigation");
```

- [ ] **Step 2: 实现详情跳转 handler**

Add to `wardrobePageConfig`:

```js
handleOpenDetail(event) {
  const dataset = getDataset(event);
  const publicID = dataset.publicId || dataset.public_id || dataset.id || "";
  if (!publicID) {
    this.setData({ errorMessage: "未找到要查看的单品" });
    return;
  }
  if (typeof wx !== "undefined" && wx.navigateTo) {
    wx.navigateTo({
      url: `/pages/wardrobe-detail/wardrobe-detail?public_id=${publicID}`
    });
  }
}
```

- [ ] **Step 3: 改 WXML 为图库主界面**

Use:

- title `我的衣服`
- `scroll-view class="category-scroll" scroll-x="true"` wrapping category chips
- `view class="gallery-grid"` for two-column image cards
- card tap binds `handleOpenDetail`
- keep `handleOpenCreate` for top `+` and `gallery-add-card`
- keep editor modal markup for create
- keep gaps section after gallery

- [ ] **Step 4: 改 WXSS 为图库视觉**

Update existing card/grid styles toward:

- larger image-first cards
- stable aspect ratio
- card overlay labels
- horizontal category scroll
- no nested page cards for the gallery itself

### Task 3: 衣服详情页

**Files:**
- Modify: `miniapp/app.json`
- Create: `miniapp/pages/wardrobe-detail/wardrobe-detail.js`
- Create: `miniapp/pages/wardrobe-detail/wardrobe-detail.wxml`
- Create: `miniapp/pages/wardrobe-detail/wardrobe-detail.wxss`
- Create: `miniapp/pages/wardrobe-detail/wardrobe-detail.json`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写失败验证**

Add app route assertion:

```js
const appJson = JSON.parse(read("app.json"));
assert(appJson.pages.includes("pages/wardrobe-detail/wardrobe-detail"), "app should register wardrobe detail page");
```

Load detail page:

```js
const detailApiCalls = [];
const wardrobeDetail = loadPage("pages/wardrobe-detail/wardrobe-detail.js", {
  getWardrobeItems: async () => ({
    items: [
      {
        public_id: "wdi_shirt",
        name: "米白衬衫",
        category: "top",
        color: "米白",
        material: "棉",
        season: "春秋",
        silhouette: "微宽松",
        user_notes: "下摆处理要干净",
        is_core: true,
        recommendation_status: "preferred",
        scene_tags: ["通勤", "见客户"],
        primary_image: { object_key: "wardrobe/wdi_shirt/main.jpg" }
      }
    ]
  }),
  updateWardrobeItem: async (publicID, payload) => {
    detailApiCalls.push(["update", publicID, payload]);
    return Object.assign({ public_id: publicID, status: "active" }, payload);
  },
  deleteWardrobeItem: async (publicID) => {
    detailApiCalls.push(["delete", publicID]);
    return { public_id: publicID };
  }
});
assert(wardrobeDetail.config, "wardrobe detail should register a Page config");
assert(typeof wardrobeDetail.config.loadWardrobeItem === "function", "wardrobe detail should load item");
assert(typeof wardrobeDetail.config.handleOpenEdit === "function", "wardrobe detail should open edit modal");
assert(typeof wardrobeDetail.config.handleSaveItem === "function", "wardrobe detail should save edits");
```

Then verify `onLoad({ public_id: "wdi_shirt" })`, edit/save, and delete return behavior.

- [ ] **Step 2: 注册页面**

Add `pages/wardrobe-detail/wardrobe-detail` to `miniapp/app.json` after `pages/wardrobe/wardrobe`.

- [ ] **Step 3: 实现详情页 JS**

Details:

- import `../../utils/api` and `../../utils/wardrobe`
- data includes `loading`, `saving`, `errorMessage`, `itemPublicID`, `item`, `editorVisible`, `draft`, `editingPublicID`
- `onLoad(options)` calls `loadWardrobeItem(options.public_id)`
- `loadWardrobeItem(publicID)` calls `api.getWardrobeItems()`, normalizes items, finds matching `public_id`
- `handleOpenEdit()` opens modal with `itemToDraft(item)`
- edit handlers mirror wardrobe page
- `handleSaveItem()` calls `api.updateWardrobeItem`, refreshes `item`, closes modal
- `handleDeleteItem()` calls `api.deleteWardrobeItem`, then `wx.switchTab({ url: "/pages/wardrobe/wardrobe" })`

- [ ] **Step 4: 实现详情页 WXML/WXSS**

Details page sections:

- back row and edit button
- large image or placeholder
- name and status
- basic meta line
- 搭配逻辑 block using `item.styleLogic`
- structured facts grid
- 最近反馈 light placeholder
- reuse bottom editor modal structure

### Task 4: 验证与收尾

**Files:**
- Verify only

- [ ] **Step 1: 运行小程序集成验证**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS.

- [ ] **Step 2: 运行 API client 验证**

Run: `cd miniapp && npm run verify:api-client`

Expected: PASS.

- [ ] **Step 3: 运行页面验证**

Run:

```bash
cd miniapp && npm run verify:report-page && npm run verify:today-ui
```

Expected: PASS.

- [ ] **Step 4: 检查空白和工作区**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors and only gallery implementation files changed.
