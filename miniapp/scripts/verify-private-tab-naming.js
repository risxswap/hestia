const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
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

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

const appJson = JSON.parse(read("app.json"));
const collectionJson = JSON.parse(read("pages/collection/collection.json"));
const wardrobeJson = JSON.parse(read("pages/wardrobe/wardrobe.json"));
const wardrobeDetailJson = JSON.parse(read("pages/wardrobe-detail/wardrobe-detail.json"));
const collectionMarkup = read("pages/collection/collection.wxml");
const collectionScript = read("pages/collection/collection.js");
const collectionStyles = read("pages/collection/collection.wxss");
const wardrobeMarkup = read("pages/wardrobe/wardrobe.wxml");
const wardrobeScript = read("pages/wardrobe/wardrobe.js");
const wardrobeDetailMarkup = read("pages/wardrobe-detail/wardrobe-detail.wxml");
const wardrobeDetailScript = read("pages/wardrobe-detail/wardrobe-detail.js");

const privateTab = appJson.tabBar && appJson.tabBar.list && appJson.tabBar.list[2];
assert(privateTab && privateTab.pagePath === "pages/collection/collection", "third tab should point to the independent collection page");
assert(privateTab.text === "私藏", "third tab should be renamed to 私藏");
assert(Array.isArray(appJson.tabBar.list), "app should define tabBar list");
appJson.tabBar.list.forEach((item) => {
  assertTabIcon(item.iconPath);
  assertTabIcon(item.selectedIconPath);
});
[
  "pages/collection/collection",
  "pages/wardrobe/wardrobe",
  "pages/hair/hair",
  "pages/makeup/makeup",
  "pages/references/references"
].forEach((pagePath) => {
  assert(appJson.pages.includes(pagePath), `app.json should register ${pagePath}`);
});

assert(collectionJson.navigationBarTitleText === "我的私藏", "collection page navigation title should be 我的私藏");
assert(wardrobeJson.navigationBarTitleText === "衣服", "wardrobe page navigation title should be 衣服");
assert(wardrobeDetailJson.navigationBarTitleText === "私藏详情", "wardrobe detail navigation title should be 私藏详情");

assert(!collectionMarkup.includes("个人长期记忆"), "collection page should remove the long-term memory eyebrow copy");
assert(!collectionMarkup.includes(">我的私藏<"), "collection page should not duplicate 我的私藏 as an in-page title");
assert(
  collectionMarkup.includes("保存衣服、发型、妆容和参考图，作为 Hestia 给你建议的依据。"),
  "collection page should explain the saved material purpose"
);
assert(collectionMarkup.includes("private-type-grid"), "collection page should render the type entry grid");
assert(collectionMarkup.includes("private-type-card"), "collection page should render private type cards");
assert(collectionMarkup.includes("recent-private-section"), "collection page should render a recent private section");
assert(collectionMarkup.includes("recent-private-grid"), "collection page should render recent private items in a grid");
assert(collectionMarkup.includes("最近收录"), "collection page should show 最近收录 heading");
["衣橱", "发型", "妆容", "参考"].forEach((label) => {
  assert(collectionScript.includes(label) || collectionMarkup.includes(label), `collection page should show ${label} type entry copy`);
});
["wardrobe", "hair", "makeup", "references"].forEach((icon) => {
  assert(collectionScript.includes(`icon: "${icon}"`), `collection page should define ${icon} type icon`);
  assert(collectionScript.includes(`assets/collection/${icon}.webp`), `collection page should define ${icon} webp image asset`);
  assertWebp(`assets/collection/${icon}.webp`, 90000);
});
assert(collectionMarkup.includes("private-type-image-icon"), "collection page should render image icons");
assert(collectionMarkup.includes('src="{{item.iconSrc}}"'), "collection page should bind icon image src from data");
assert(!collectionMarkup.includes("icon-line"), "collection page should not render CSS line icon pieces");
assert(!collectionStyles.includes(".icon-line"), "collection styles should not draw icons with CSS lines");
assert(collectionStyles.includes("min-height: 184rpx"), "collection type cards should reserve a full-height row");
assert(collectionStyles.includes("flex: 0 0 132rpx"), "collection type icon column should be wider");
assert(collectionStyles.includes("height: 132rpx"), "collection type icon should occupy the row height");
assert(collectionStyles.includes("align-self: center"), "collection type icon should stay vertically centered in the row");
assert(collectionStyles.includes("flex: 1"), "collection type copy should remain the right-side flexible column");
assert(collectionScript.includes("getCollectionSummary"), "collection page should load collection summary through its own API");
assert(collectionScript.includes("handleOpenType"), "collection page should own type navigation");
assert(collectionScript.includes("handleOpenRecent"), "collection page should own recent item navigation");
assert(!collectionMarkup.includes("category-scroll"), "collection home should not render the clothing category strip");

assert(wardrobeMarkup.includes("衣服"), "wardrobe page should use 衣服 as page title");
assert(wardrobeMarkup.includes("新增衣服"), "wardrobe page should use 新增衣服 for add entry");
assert(!wardrobeMarkup.includes("我的衣服"), "private page should not keep old 我的衣服 title");
assert(!wardrobeMarkup.includes("正在读取衣橱"), "private page should not expose old 衣橱 loading copy");
assert(!wardrobeMarkup.includes("private-type-grid"), "wardrobe page should not render collection type grid");
assert(!wardrobeScript.includes("handleOpenPrivateType"), "wardrobe page should not own collection type navigation");

assert(collectionScript.includes("读取私藏失败"), "collection page script should use collection failure copy");
assert(wardrobeScript.includes("读取衣服失败"), "wardrobe page script should use clothes failure copy");
assert(wardrobeScript.includes("读取补齐建议失败"), "wardrobe page script should use gap failure copy");
assert(wardrobeDetailMarkup.includes("返回私藏"), "detail page should return to 私藏");
assert(!wardrobeDetailMarkup.includes("返回衣橱"), "detail page should not return to 衣橱");
assert(wardrobeDetailScript.includes("读取私藏详情失败"), "detail page script should use private detail failure copy");

console.log("private tab naming verification passed");
