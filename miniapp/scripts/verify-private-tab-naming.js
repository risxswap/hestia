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

const appJson = JSON.parse(read("app.json"));
const collectionJson = JSON.parse(read("pages/collection/collection.json"));
const wardrobeJson = JSON.parse(read("pages/wardrobe/wardrobe.json"));
const wardrobeDetailJson = JSON.parse(read("pages/wardrobe-detail/wardrobe-detail.json"));
const collectionMarkup = read("pages/collection/collection.wxml");
const collectionScript = read("pages/collection/collection.js");
const wardrobeMarkup = read("pages/wardrobe/wardrobe.wxml");
const wardrobeScript = read("pages/wardrobe/wardrobe.js");
const wardrobeDetailMarkup = read("pages/wardrobe-detail/wardrobe-detail.wxml");
const wardrobeDetailScript = read("pages/wardrobe-detail/wardrobe-detail.js");

const privateTab = appJson.tabBar && appJson.tabBar.list && appJson.tabBar.list[2];
assert(privateTab && privateTab.pagePath === "pages/collection/collection", "third tab should point to the independent collection page");
assert(privateTab.text === "私藏", "third tab should be renamed to 私藏");
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

assert(collectionMarkup.includes("我的私藏"), "collection page should use 我的私藏 as page title");
assert(
  collectionMarkup.includes("保存衣服、发型、妆容和参考图，作为 Hestia 给你建议的依据。"),
  "collection page should explain the saved material purpose"
);
assert(collectionMarkup.includes("private-type-grid"), "collection page should render the type entry grid");
assert(collectionMarkup.includes("private-type-card"), "collection page should render private type cards");
assert(collectionMarkup.includes("recent-private-section"), "collection page should render a recent private section");
assert(collectionMarkup.includes("recent-private-grid"), "collection page should render recent private items in a grid");
assert(collectionMarkup.includes("最近收录"), "collection page should show 最近收录 heading");
["衣服", "发型", "妆容", "参考"].forEach((label) => {
  assert(collectionScript.includes(label) || collectionMarkup.includes(label), `collection page should show ${label} type entry copy`);
});
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
