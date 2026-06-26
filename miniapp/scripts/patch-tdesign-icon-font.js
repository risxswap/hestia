const fs = require("fs");
const https = require("https");
const path = require("path");

const root = path.resolve(__dirname, "..");
const fontUrl = "https://tdesign.gtimg.com/icon/0.4.2/fonts/t.woff";
const targetFiles = [
  "node_modules/tdesign-miniprogram/miniprogram_dist/icon/icon.wxss",
  "miniprogram_npm/tdesign-miniprogram/icon/icon.wxss"
];

function download(url) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (response) => {
        if (response.statusCode !== 200) {
          reject(new Error(`download ${url} failed with status ${response.statusCode}`));
          response.resume();
          return;
        }

        const chunks = [];
        response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => resolve(Buffer.concat(chunks)));
      })
      .on("error", reject);
  });
}

function patchIconWxss(filePath, fontBase64) {
  if (!fs.existsSync(filePath)) {
    return false;
  }

  const source = fs.readFileSync(filePath, "utf8");
  if (!source.includes("tdesign.gtimg.com/icon")) {
    return false;
  }

  const fontFace = `@font-face{font-family:t;src:url(data:font/woff;base64,${fontBase64}) format('woff');font-weight:400;font-style:normal;}`;
  const patched = source.replace(/@font-face\{font-family:t;src:[^}]+font-weight:400;font-style:normal;\}/, fontFace);

  if (patched === source) {
    throw new Error(`failed to replace @font-face in ${filePath}`);
  }

  fs.writeFileSync(filePath, patched);
  return true;
}

async function main() {
  const font = await download(fontUrl);
  const fontBase64 = font.toString("base64");
  const patchedFiles = [];

  for (const relativePath of targetFiles) {
    const filePath = path.join(root, relativePath);
    if (patchIconWxss(filePath, fontBase64)) {
      patchedFiles.push(relativePath);
    }
  }

  if (!patchedFiles.length) {
    console.log("tdesign icon font already patched");
    return;
  }

  console.log(`tdesign icon font patched: ${patchedFiles.join(", ")}`);
}

main().catch((error) => {
  if (process.env.npm_lifecycle_event === "postinstall") {
    console.warn(`skip tdesign icon font patch: ${error.message}`);
    return;
  }

  console.error(error);
  process.exit(1);
});
