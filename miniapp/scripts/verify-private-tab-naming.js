const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function assertWebp(relativePath, maxBytes) {
  const filePath = path.join(root, relativePath);
  assert(fs.existsSync(filePath), `${relativePath} should exist`);
  const buffer = fs.readFileSync(filePath);
  assert(buffer.subarray(0, 4).toString("ascii") === "RIFF", `${relativePath} should be a RIFF container`);
  assert(buffer.subarray(8, 12).toString("ascii") === "WEBP", `${relativePath} should be a WebP image`);
  assert(buffer.length <= maxBytes, `${relativePath} should be <= ${maxBytes} bytes, got ${buffer.length}`);
}

function assertTabIcon(relativePath) {
  const filePath = path.join(root, relativePath);
  assert(/\.(png|jpe?g)$/i.test(relativePath), `${relativePath} should use a WeChat tabBar supported format`);
  assert(fs.existsSync(filePath), `${relativePath} should exist`);
  const buffer = fs.readFileSync(filePath);
  const isPng = buffer.subarray(0, 8).toString("hex") === "89504e470d0a1a0a";
  const isJpeg = buffer[0] === 0xff && buffer[1] === 0xd8 && buffer[2] === 0xff;
  assert(isPng || isJpeg, `${relativePath} should be a PNG or JPEG image`);
  assert(buffer.length <= 20000, `${relativePath} should be <= 20000 bytes, got ${buffer.length}`);
}

const appJson = JSON.parse(read("app.json"));
const projectJson = JSON.parse(read("project.config.json"));
const collectionJson = JSON.parse(read("pages/collection/collection.json"));
const clothesJson = JSON.parse(read("pages/clothes/list.json"));
const clothesDetailJson = JSON.parse(read("pages/clothes/detail.json"));
const clothesEditJson = JSON.parse(read("pages/clothes/edit.json"));
const profileJson = JSON.parse(read("pages/profile/index.json"));
const collectionMarkup = read("pages/collection/collection.wxml");
const collectionScript = read("pages/collection/collection.js");
const collectionStyles = read("pages/collection/collection.wxss");
const clothesMarkup = read("pages/clothes/list.wxml");
const clothesScript = read("pages/clothes/list.js");
const clothesDetailMarkup = read("pages/clothes/detail.wxml");
const clothesDetailScript = read("pages/clothes/detail.js");
const clothesEditMarkup = read("pages/clothes/edit.wxml");
const profileMarkup = read("pages/profile/index.wxml");

const privateTab = appJson.tabBar && appJson.tabBar.list && appJson.tabBar.list[2];
assert(privateTab && privateTab.pagePath === "pages/collection/collection", "third tab should point to the independent collection page");
assert(privateTab.text === "私藏", "third tab should be 私藏");
assert(appJson.tabBar.list[3].pagePath === "pages/profile/index", "profile tab should point to pages/profile/index");
appJson.tabBar.list.forEach((item) => {
  assertTabIcon(item.iconPath);
  assertTabIcon(item.selectedIconPath);
});
assert(
  projectJson.setting && projectJson.setting.ignoreDevUnusedFiles !== true,
  "project config should not ignore dev unused files after page route renames",
);
assert(
  projectJson.packOptions && Array.isArray(projectJson.packOptions.ignore) && projectJson.packOptions.ignore.length === 0,
  "project config should not exclude page files from package",
);

[
  "pages/collection/collection",
  "pages/clothes/list",
  "pages/clothes/detail",
  "pages/clothes/edit",
  "pages/hair/list",
  "pages/hair/detail",
  "pages/hair/edit",
  "pages/makeup/list",
  "pages/makeup/detail",
  "pages/makeup/edit",
  "pages/profile/index",
  "pages/profile/edit",
  "pages/preferences/edit",
  "pages/memory/index",
  "pages/privacy/index"
].forEach((pagePath) => {
  assert(appJson.pages.includes(pagePath), `app.json should register ${pagePath}`);
  [".js", ".json", ".wxml", ".wxss"].forEach((extension) => {
    assert(fs.existsSync(path.join(root, `${pagePath}${extension}`)), `${pagePath}${extension} should exist`);
  });
});

[
  "pages/wardrobe/wardrobe",
  "pages/wardrobe-detail/wardrobe-detail",
  "pages/wardrobe-edit/wardrobe-edit",
  "pages/references/references",
  "pages/profile/profile"
].forEach((pagePath) => {
  assert(!appJson.pages.includes(pagePath), `app.json should not register old page ${pagePath}`);
});

assert(collectionJson.navigationBarTitleText === "我的私藏", "collection navigation title should be 我的私藏");
assert(clothesJson.navigationBarTitleText === "衣服", "clothes list title should be 衣服");
assert(clothesDetailJson.navigationBarTitleText === "私藏详情", "clothes detail title should be 私藏详情");
assert(clothesEditJson.navigationBarTitleText === "编辑衣服", "clothes edit title should be 编辑衣服");
assert(profileJson.navigationBarTitleText === "我的", "profile index title should be 我的");

assert(collectionMarkup.includes("保存衣服、发型和妆容"), "collection page should describe clothes/hair/makeup only");
["衣服", "发型", "妆容"].forEach((label) => {
  assert(collectionScript.includes(label) || collectionMarkup.includes(label), `collection page should show ${label}`);
});
["clothes", "hair", "makeup"].forEach((icon) => {
  assert(collectionScript.includes(`icon: "${icon}"`), `collection page should define ${icon} icon`);
  assert(collectionScript.includes(`assets/collection/${icon}.webp`), `collection page should define ${icon} webp asset`);
  assertWebp(`assets/collection/${icon}.webp`, 90000);
});
assert(!collectionScript.includes("references"), "collection should remove references type");
assert(!collectionMarkup.includes("参考图"), "collection should remove reference copy");
assert(!fs.existsSync(path.join(root, "assets/collection/references.webp")), "references asset should be removed");
assert(collectionMarkup.includes("private-type-grid"), "collection page should render type grid");
assert(collectionMarkup.includes("recent-private-section"), "collection page should render recent section");
assert(!collectionStyles.includes(".icon-line"), "collection styles should not draw CSS line icons");

assert(clothesMarkup.includes("新增衣服"), "clothes list should keep lightweight create entry");
assert(clothesMarkup.includes("modal-sheet"), "clothes list should use create modal");
assert(clothesMarkup.includes('bind:add="handleImageUpload"'), "clothes list upload should use add event");
assert(!clothesMarkup.includes('bind:success="handleImageUpload"'), "clothes list upload should not bind success to upload handler");
assert(clothesScript.includes("createClothesItem"), "clothes list should create through clothes API");
assert(clothesScript.includes("/pages/clothes/detail"), "clothes list should navigate to detail route");
assert(clothesScript.includes("source=clothes"), "clothes list should pass source when navigating to detail");
assert(clothesDetailMarkup.includes("编辑"), "clothes detail should expose edit action");
assert(clothesDetailMarkup.includes("<swiper"), "clothes detail should support swiping between images");
assert(clothesDetailMarkup.includes("imageSlides"), "clothes detail swiper should render normalized image slides");
assert(clothesDetailMarkup.includes("{{backLabel}}"), "clothes detail should bind back label from source state");
assert(clothesDetailScript.includes('backLabel: "返回私藏"'), "clothes detail should default back label to 私藏");
assert(clothesDetailScript.includes('backUrl: "/pages/collection/collection"'), "clothes detail should default back url to collection");
assert(clothesDetailScript.includes('backMode: "switchTab"'), "clothes detail should default back mode to tab switching");
assert(clothesDetailScript.includes('source === "clothes"'), "clothes detail should detect clothes source");
assert(clothesDetailScript.includes('"返回衣服"'), "clothes detail should support returning to clothes");
assert(clothesDetailScript.includes('"/pages/clothes/list"'), "clothes detail should support clothes back url");
assert(clothesDetailScript.includes('"navigateBack"'), "clothes detail should navigate back for non-tab clothes source");
assert(clothesDetailScript.includes("/pages/clothes/edit"), "clothes detail should navigate to edit route");
assert(!clothesDetailMarkup.includes("<input"), "clothes detail should not include inputs");
assert(!clothesDetailMarkup.includes("<textarea"), "clothes detail should not include textareas");
assert(!clothesDetailMarkup.includes("保存"), "clothes detail should not include save action");
assert(clothesEditMarkup.includes("<input"), "clothes edit should include form inputs");
assert(clothesEditMarkup.includes("保存"), "clothes edit should include save action");
assert(clothesEditMarkup.includes('bind:add="handleImageUpload"'), "clothes edit upload should use add event");
assert(!clothesEditMarkup.includes('bind:success="handleImageUpload"'), "clothes edit upload should not bind success to upload handler");

assert(!profileMarkup.includes("<input"), "profile index should not include profile form inputs");
assert(!profileMarkup.includes("<textarea"), "profile index should not include textareas");
assert(!profileMarkup.includes("我的形象档案"), "profile index should not show top identity title");
assert(!profileMarkup.includes("用户："), "profile index should not show user id copy");
assert(!profileMarkup.includes("本地开发用户"), "profile index should not show local user fallback copy");
assert(!profileMarkup.includes("onboarding"), "profile index should not show onboarding status copy");
assert(!profileMarkup.includes("补充档案"), "profile index should not show supplemental profile action");
assert(!profileMarkup.includes("handleStartOnboarding"), "profile index should not bind onboarding action");
assert(!profileMarkup.includes("档案摘要"), "profile index should not duplicate profile summary");
assert(!profileMarkup.includes("长期记忆摘要"), "profile index should not duplicate memory summary");
assert(profileMarkup.includes('class="quick-list"'), "profile index should render entries as rows");

console.log("private tab naming verification passed");
