const path = require("path");

const root = path.resolve(__dirname, "..");
const pagePath = path.join(root, "pages/advisor/advisor.js");
const apiPath = path.join(root, "utils/api.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

let pageConfig = null;
const originalPage = global.Page;
const originalApiCache = require.cache[require.resolve(apiPath)];
const apiCalls = [];

async function main() {
try {
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: {
      getCurrentAdviceDraft: async () => null,
      uploadFileToQiniu: async (file, options) => {
        apiCalls.push({ name: "uploadFileToQiniu", file, options });
        return { asset_public_id: "ast_chat_photo" };
      },
      streamAgentChat: (payload) => {
        apiCalls.push({ name: "streamAgentChat", payload });
        return {
          promise: Promise.resolve([]),
          abort() {}
        };
      },
      confirmAdviceDraft: async () => ({}),
      discardAdviceDraft: async () => ({})
    }
  };
  global.Page = (config) => {
    pageConfig = config;
  };
  delete require.cache[require.resolve(pagePath)];
  const advisor = require(pagePath);

  assert(pageConfig, "advisor page should call Page()");
  [
    "handleSend",
    "handleStop",
    "handleConfirmDraft",
    "handleContinueDraft",
    "handleDiscardDraft",
    "restoreCurrentDraft",
    "handleFileSelect"
  ].forEach((name) => {
    assert(typeof pageConfig[name] === "function", `advisor page should define ${name}`);
  });

  const draft = advisor.normalizeDraftCard({
    draft_public_id: "drf_test",
    revision_no: 3,
    status: "draft",
    scene_label: "见客户",
    sections: [{
      public_id: "ads_outfit",
      section_type: "outfit",
      content_json: {
        title: "清爽通勤",
        summary: "米白衬衫搭直筒裤",
        why_text: "更利落",
        avoid_text: "避免过紧",
        alternative_text: "可换乐福鞋"
      }
    }]
  });

  assert(draft.draft_public_id === "drf_test", "draft public id should be kept");
  assert(draft.revision_no === 3, "draft revision should be kept");
  assert(draft.sections.length === 1, "draft sections should be normalized");
  assert(draft.sections[0].label === "穿搭", "outfit section should use Chinese label");
  assert(draft.sections[0].title === "清爽通勤", "section title should map from content_json");
  assert(advisor.sectionLabel("hair") === "发型", "hair section label mismatch");
  assert(advisor.sectionLabel("makeup") === "妆容", "makeup section label mismatch");

  const page = Object.assign({}, pageConfig, {
    data: JSON.parse(JSON.stringify(pageConfig.data)),
    setData(patch, callback) {
      this.data = Object.assign({}, this.data, patch);
      if (typeof callback === "function") {
        callback();
      }
    }
  });
  await awaitMaybe(page.handleFileSelect.call(page, {
    detail: {
      files: [{ url: "/tmp/chat-look.jpg", size: 2048, width: 640, height: 960 }]
    }
  }));
  const uploadCall = apiCalls.find((call) => call.name === "uploadFileToQiniu");
  assert(uploadCall, "advisor page should upload selected chat image");
  assert(uploadCall.options.assetType === "chat_image", "advisor chat upload should use chat_image asset type");
  const streamCall = apiCalls.find((call) => call.name === "streamAgentChat");
  assert(streamCall, "advisor page should send uploaded image to agent");
  assert(streamCall.payload.text === "我想用这张照片提问", "advisor page should send photo prompt text");
  assert(streamCall.payload.assetRefs[0].asset_public_id === "ast_chat_photo", "advisor page should send uploaded asset ref");
  const userPhotoMessage = page.data.messages.find((message) => message.role === "user" && message.images && message.images.length);
  assert(userPhotoMessage.images[0].url === "/tmp/chat-look.jpg", "advisor page should render selected image preview");

  console.log("advisor page verification passed");
} finally {
  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }
  if (originalApiCache) {
    require.cache[require.resolve(apiPath)] = originalApiCache;
  } else {
    delete require.cache[require.resolve(apiPath)];
  }
}
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});

function awaitMaybe(value) {
  if (value && typeof value.then === "function") {
    return value;
  }
  return Promise.resolve(value);
}
