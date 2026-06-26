const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const iconWxssPath = path.join(root, "miniprogram_npm/tdesign-miniprogram/icon/icon.wxss");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

assert(fs.existsSync(iconWxssPath), "请先在微信开发者工具中执行：工具 -> 构建 npm");

const iconWxss = fs.readFileSync(iconWxssPath, "utf8");

assert(
  !iconWxss.includes("tdesign.gtimg.com/icon"),
  "TDesign icon.wxss should not load icon fonts from tdesign.gtimg.com"
);
assert(
  iconWxss.includes("data:font/woff;base64,"),
  "TDesign icon.wxss should inline the icon font as a data URI"
);

console.log("tdesign icon font verification passed");
