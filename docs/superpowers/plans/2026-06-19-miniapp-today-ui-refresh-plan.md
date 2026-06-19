# Miniapp Today UI Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将小程序首页升级为 `今日` 入口，首屏直接呈现“今天推荐你这样穿”的生活化私人衣橱顾问体验。

**Architecture:** 保持现有微信小程序结构，不引入新框架。`miniapp/utils/mock.js` 提供今日推荐演示数据，`miniapp/pages/home/*` 渲染今日页，`miniapp/app.json` 更新 Tab 文案，验证通过静态脚本检查关键结构和文案。

**Tech Stack:** 微信小程序 WXML/WXSS/JS、CommonJS mock 数据、Node.js 静态验证。

---

## File Structure

- Modify: `miniapp/app.json`
  - 将首个 tab 文案从 `首页` 改为 `今日`。
- Modify: `miniapp/utils/mock.js`
  - 保留现有 `actionItems`，新增 `todayRecommendation`、`todayPlanSections`、`feedbackOptions`，供今日页渲染。
- Modify: `miniapp/pages/home/home.js`
  - 从 mock 数据读取今日推荐、展开方案和反馈选项。
- Modify: `miniapp/pages/home/home.wxml`
  - 将旧行动清单页面改为今日推荐首屏、方案详情和反馈入口。
- Modify: `miniapp/pages/home/home.wxss`
  - 实现暖米白、深绿、金棕、柔和照片氛围、胶囊 CTA、可扫读方案模块。
- Create: `miniapp/scripts/verify-today-ui.js`
  - 静态验证今日页关键文案、数据字段和页面结构。
- Modify: `miniapp/package.json`
  - 新增 `verify:today-ui` 脚本。

## Task 1: Static Verification Harness

**Files:**
- Create: `miniapp/scripts/verify-today-ui.js`
- Modify: `miniapp/package.json`

- [ ] **Step 1: Write the failing verification script**

Create `miniapp/scripts/verify-today-ui.js` with:

```js
const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assertIncludes(file, content, expected) {
  if (!content.includes(expected)) {
    throw new Error(`${file} should include ${expected}`);
  }
}

const appJson = JSON.parse(read("app.json"));
const mockSource = read("utils/mock.js");
const homeJs = read("pages/home/home.js");
const homeWxml = read("pages/home/home.wxml");
const homeWxss = read("pages/home/home.wxss");

if (appJson.tabBar.list[0].text !== "今日") {
  throw new Error(`first tab should be 今日, got ${appJson.tabBar.list[0].text}`);
}

assertIncludes("utils/mock.js", mockSource, "todayRecommendation");
assertIncludes("utils/mock.js", mockSource, "todayPlanSections");
assertIncludes("utils/mock.js", mockSource, "feedbackOptions");
assertIncludes("pages/home/home.js", homeJs, "todayRecommendation");
assertIncludes("pages/home/home.js", homeJs, "todayPlanSections");
assertIncludes("pages/home/home.js", homeJs, "feedbackOptions");
assertIncludes("pages/home/home.wxml", homeWxml, "今日推荐");
assertIncludes("pages/home/home.wxml", homeWxml, "照这个穿");
assertIncludes("pages/home/home.wxml", homeWxml, "换个场景");
assertIncludes("pages/home/home.wxml", homeWxml, "为什么适合今天");
assertIncludes("pages/home/home.wxml", homeWxml, "bind:tap=\"handleFeedback\"");
assertIncludes("pages/home/home.wxss", homeWxss, "#245d4f");
assertIncludes("pages/home/home.wxss", homeWxss, "today-visual");
assertIncludes("pages/home/home.wxss", homeWxss, "feedback-chip");

console.log("today ui verification passed");
```

- [ ] **Step 2: Add the npm script**

Update `miniapp/package.json` scripts to:

```json
"scripts": {
  "build:npm": "echo \"请在微信开发者工具中执行：工具 -> 构建 npm\"",
  "verify:today-ui": "node scripts/verify-today-ui.js"
}
```

- [ ] **Step 3: Run verification to confirm RED**

Run:

```bash
cd miniapp && npm run verify:today-ui
```

Expected: FAIL with `first tab should be 今日` or a missing `todayRecommendation` assertion.

## Task 2: Data and Tab Copy

**Files:**
- Modify: `miniapp/app.json`
- Modify: `miniapp/utils/mock.js`
- Modify: `miniapp/pages/home/home.js`

- [ ] **Step 1: Update the tab label**

In `miniapp/app.json`, change only the first tab text:

```json
"text": "今日"
```

- [ ] **Step 2: Add today recommendation mock data**

In `miniapp/utils/mock.js`, keep `actionItems` and add:

```js
const todayRecommendation = {
  context: "周五 22° / 见客户后的晚餐",
  label: "今日推荐",
  title: "浅外套 + 直筒裤，轻松但有精神",
  summary: "适合今天从客户场景切到晚餐场景，保留干净线条，也不会显得太用力。",
  visualTitle: "柔和浅色层次",
  visualMeta: "用你常穿的浅外套做主角",
  primaryAction: "照这个穿",
  secondaryAction: "换个场景"
};

const todayPlanSections = [
  {
    title: "为什么适合今天",
    body: "浅色短外套能提亮上半身，直筒裤保持利落感，适合需要亲和但不松散的场合。"
  },
  {
    title: "发型方向",
    body: "保留额头附近的呼吸感，发尾不要压得太厚，让整体更轻。"
  },
  {
    title: "今天不优先",
    body: "不优先软塌针织和过甜的裙装。如果想更温柔，可以换成有筋骨感的浅色开衫。"
  },
  {
    title: "可替换单品",
    body: "没有浅外套时，用米白衬衫外搭薄马甲；鞋子优先低跟单鞋或干净乐福鞋。"
  }
];

const feedbackOptions = [
  "照这个穿",
  "不喜欢",
  "换正式一点",
  "换轻松一点",
  "我实际这样穿了"
];
```

Export all data:

```js
module.exports = {
  actionItems,
  todayRecommendation,
  todayPlanSections,
  feedbackOptions
};
```

- [ ] **Step 3: Wire data into the home page**

Replace `miniapp/pages/home/home.js` with:

```js
const {
  actionItems,
  feedbackOptions,
  todayPlanSections,
  todayRecommendation
} = require("../../utils/mock");

Page({
  data: {
    updatedAt: "2026-06-19",
    actionItems,
    feedbackOptions,
    todayPlanSections,
    todayRecommendation,
    memoryToast: ""
  },

  handlePrimaryAction() {
    this.setData({
      memoryToast: "已记住：你今天更喜欢利落但不强势的感觉。"
    });
  },

  handleSceneChange() {
    this.setData({
      memoryToast: "可以告诉我新场景，我会换一套更贴近的建议。"
    });
  },

  handleFeedback(event) {
    const value = event.currentTarget.dataset.value;
    this.setData({
      memoryToast: `已收到：${value}`
    });
  }
});
```

- [ ] **Step 4: Run verification and confirm remaining GREEN/expected failures**

Run:

```bash
cd miniapp && npm run verify:today-ui
```

Expected: still FAIL because WXML/WXSS structure is not implemented yet.

## Task 3: Today Page Markup and Styling

**Files:**
- Modify: `miniapp/pages/home/home.wxml`
- Modify: `miniapp/pages/home/home.wxss`

- [ ] **Step 1: Replace home markup**

Replace `miniapp/pages/home/home.wxml` with:

```xml
<view class="page home-page today-page">
  <view class="today-hero">
    <view class="today-topline">
      <text class="today-brand">Hestia</text>
      <text class="today-context">{{todayRecommendation.context}}</text>
    </view>

    <view class="today-label">{{todayRecommendation.label}}</view>
    <text class="today-title">{{todayRecommendation.title}}</text>
    <text class="today-summary">{{todayRecommendation.summary}}</text>

    <view class="today-visual" aria-label="{{todayRecommendation.visualTitle}}">
      <view class="visual-swatch visual-swatch--light"></view>
      <view class="visual-swatch visual-swatch--green"></view>
      <view class="visual-swatch visual-swatch--gold"></view>
      <view class="visual-copy">
        <text class="visual-title">{{todayRecommendation.visualTitle}}</text>
        <text class="visual-meta">{{todayRecommendation.visualMeta}}</text>
      </view>
    </view>

    <view class="today-actions">
      <button class="primary-button today-button" bind:tap="handlePrimaryAction">
        {{todayRecommendation.primaryAction}}
      </button>
      <button class="secondary-button today-button" bind:tap="handleSceneChange">
        {{todayRecommendation.secondaryAction}}
      </button>
    </view>
  </view>

  <view class="today-section">
    <view class="section-head">
      <text class="section-kicker">顾问方案</text>
      <text class="section-title">为什么适合今天</text>
    </view>

    <block wx:for="{{todayPlanSections}}" wx:key="title">
      <view class="plan-item">
        <text class="plan-title">{{item.title}}</text>
        <text class="plan-body">{{item.body}}</text>
      </view>
    </block>
  </view>

  <view class="today-section feedback-section">
    <view class="section-head">
      <text class="section-kicker">让建议进化</text>
      <text class="section-title">你的反馈我会记住</text>
    </view>

    <view class="feedback-grid">
      <block wx:for="{{feedbackOptions}}" wx:key="*this">
        <button class="feedback-chip" data-value="{{item}}" bind:tap="handleFeedback">
          {{item}}
        </button>
      </block>
    </view>

    <text wx:if="{{memoryToast}}" class="memory-toast">{{memoryToast}}</text>
  </view>
</view>
```

- [ ] **Step 2: Replace home styles**

Replace `miniapp/pages/home/home.wxss` with:

```css
.home-page {
  min-height: calc(100vh - 64rpx);
}

.today-page {
  padding-top: 28rpx;
  background:
    linear-gradient(180deg, rgba(255, 250, 242, 0.96), rgba(242, 234, 223, 0.98)),
    repeating-linear-gradient(135deg, rgba(36, 93, 79, 0.035) 0, rgba(36, 93, 79, 0.035) 1rpx, transparent 1rpx, transparent 18rpx);
}

.today-hero,
.today-section {
  border: 1rpx solid rgba(222, 213, 199, 0.92);
  border-radius: 28rpx;
  background: rgba(255, 253, 248, 0.9);
  box-shadow: 0 18rpx 48rpx rgba(71, 56, 36, 0.1);
}

.today-hero {
  padding: 30rpx;
}

.today-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  color: #756b60;
  font-size: 24rpx;
}

.today-brand {
  color: #27231f;
  font-weight: 700;
}

.today-context {
  min-width: 0;
  text-align: right;
}

.today-label {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  height: 46rpx;
  margin-top: 30rpx;
  padding: 0 20rpx;
  border-radius: 999rpx;
  color: #245d4f;
  background: #e8f0eb;
  font-size: 23rpx;
  font-weight: 600;
}

.today-title {
  display: block;
  margin-top: 18rpx;
  color: #27231f;
  font-size: 46rpx;
  font-weight: 800;
  line-height: 1.22;
}

.today-summary {
  display: block;
  margin-top: 18rpx;
  color: #665d52;
  font-size: 28rpx;
  line-height: 1.68;
}

.today-visual {
  position: relative;
  min-height: 228rpx;
  margin-top: 28rpx;
  overflow: hidden;
  border: 1rpx solid rgba(195, 178, 153, 0.5);
  border-radius: 32rpx;
  background:
    linear-gradient(120deg, rgba(36, 93, 79, 0.08), rgba(174, 126, 71, 0.08)),
    linear-gradient(135deg, #d8c7ad, #f9f0e5 44%, #8f9e86);
}

.visual-swatch {
  position: absolute;
  border-radius: 999rpx;
  filter: blur(1rpx);
}

.visual-swatch--light {
  top: 38rpx;
  left: 34rpx;
  width: 96rpx;
  height: 96rpx;
  background: rgba(255, 250, 242, 0.78);
}

.visual-swatch--green {
  right: 48rpx;
  bottom: 34rpx;
  width: 128rpx;
  height: 128rpx;
  background: rgba(36, 93, 79, 0.34);
}

.visual-swatch--gold {
  top: 52rpx;
  right: 136rpx;
  width: 74rpx;
  height: 74rpx;
  background: rgba(174, 126, 71, 0.28);
}

.visual-copy {
  position: absolute;
  right: 24rpx;
  bottom: 24rpx;
  left: 24rpx;
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  padding: 20rpx;
  border-radius: 24rpx;
  background: rgba(255, 253, 248, 0.72);
}

.visual-title {
  color: #27231f;
  font-size: 27rpx;
  font-weight: 700;
}

.visual-meta {
  color: #6b6258;
  font-size: 23rpx;
}

.today-actions {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16rpx;
  margin-top: 26rpx;
}

.today-button {
  width: 100%;
  height: 76rpx;
  margin: 0;
  border-radius: 999rpx;
  font-size: 27rpx;
  font-weight: 650;
  line-height: 76rpx;
}

.today-button::after,
.feedback-chip::after {
  border: 0;
}

.primary-button {
  color: #fffdf8;
  background: #245d4f;
}

.secondary-button {
  color: #245d4f;
  background: #f1eadf;
}

.today-section {
  margin-top: 24rpx;
  padding: 28rpx;
}

.section-head {
  display: flex;
  flex-direction: column;
  gap: 8rpx;
}

.section-kicker {
  color: #a9783a;
  font-size: 23rpx;
  font-weight: 650;
}

.section-title {
  color: #27231f;
  font-size: 34rpx;
  font-weight: 750;
}

.plan-item {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  padding: 24rpx 0;
  border-top: 1rpx solid rgba(222, 213, 199, 0.82);
}

.plan-item:first-of-type {
  margin-top: 12rpx;
}

.plan-title {
  color: #27231f;
  font-size: 29rpx;
  font-weight: 700;
}

.plan-body {
  color: #665d52;
  font-size: 27rpx;
  line-height: 1.68;
}

.feedback-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 14rpx;
  margin-top: 22rpx;
}

.feedback-chip {
  display: inline-flex;
  align-items: center;
  width: auto;
  height: 58rpx;
  margin: 0;
  padding: 0 22rpx;
  border: 1rpx solid rgba(36, 93, 79, 0.16);
  border-radius: 999rpx;
  color: #245d4f;
  background: #fffaf2;
  font-size: 25rpx;
  font-weight: 600;
  line-height: 58rpx;
}

.memory-toast {
  display: block;
  margin-top: 22rpx;
  padding: 18rpx 20rpx;
  border-radius: 20rpx;
  color: #245d4f;
  background: #e8f0eb;
  font-size: 25rpx;
  line-height: 1.5;
}
```

- [ ] **Step 3: Run verification to confirm GREEN**

Run:

```bash
cd miniapp && npm run verify:today-ui
```

Expected: PASS with `today ui verification passed`.

## Task 4: Final Verification

**Files:**
- No production edits unless verification reveals a defect.

- [ ] **Step 1: Run package script verification**

Run:

```bash
cd miniapp && npm run verify:today-ui
```

Expected: PASS with `today ui verification passed`.

- [ ] **Step 2: Run JSON syntax validation**

Run:

```bash
node -e "JSON.parse(require('fs').readFileSync('miniapp/app.json','utf8')); JSON.parse(require('fs').readFileSync('miniapp/package.json','utf8')); console.log('json ok')"
```

Expected: PASS with `json ok`.

- [ ] **Step 3: Check git status and diff scope**

Run:

```bash
git status --short
git diff -- miniapp/app.json miniapp/package.json miniapp/utils/mock.js miniapp/pages/home/home.js miniapp/pages/home/home.wxml miniapp/pages/home/home.wxss miniapp/scripts/verify-today-ui.js
```

Expected: only intended files are changed for this implementation; existing unrelated dirty files remain unstaged and untouched.

## Self-Review

- Spec coverage: Tab 文案、今日页首屏、柔和照片氛围、方案展开、反馈入口、视觉 token 都有任务覆盖。
- Placeholder scan: 无 TBD/TODO/待定。
- Type consistency: `todayRecommendation`、`todayPlanSections`、`feedbackOptions` 在 mock、home.js、WXML 和验证脚本中命名一致。
