const path = require("path");

const root = path.resolve(__dirname, "..");
const pagePath = path.join(root, "pages/advisor/advisor.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

let pageConfig = null;
const originalPage = global.Page;

try {
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
    "handleDiscardDraft"
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

  console.log("advisor page verification passed");
} finally {
  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }
}
