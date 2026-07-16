const path = require("path");
const fs = require("fs");

const root = path.resolve(__dirname, "..");
const pagePath = path.join(root, "pages/advisor/advisor.js");
const apiPath = path.join(root, "utils/api.js");
const pageJSONPath = path.join(root, "pages/advisor/advisor.json");
const pageWXMLPath = path.join(root, "pages/advisor/advisor.wxml");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

let pageConfig = null;
const originalPage = global.Page;
const originalWx = global.wx;
const originalApiCache = require.cache[require.resolve(apiPath)];
const apiCalls = [];

async function main() {
try {
  const advisorJSON = fs.readFileSync(pageJSONPath, "utf8");
  const advisorWXML = fs.readFileSync(pageWXMLPath, "utf8");
  assert(!advisorJSON.includes("t-chat-sender"), "advisor page must not depend on t-chat-sender and its incompatible attachments component");
  assert(!advisorWXML.includes("<t-chat-sender"), "advisor markup must not render t-chat-sender");
  assert(advisorWXML.includes('bind:tap="handleChooseImage"'), "advisor markup should provide a native image picker entry");
  assert(advisorWXML.includes('class="agent-process__summary"'), "advisor markup should render an agent process summary control");
  assert(advisorWXML.includes('bind:tap="handleToggleProcess"'), "agent process summary should be expandable");
  assert(!advisorWXML.includes("正在处理"), "agent process UI should show a stage summary instead of a generic processing label");
  assert(advisorJSON.includes('"t-chat-markdown"'), "advisor page should register TDesign markdown rendering");
  assert(advisorWXML.includes("<t-chat-markdown"), "advisor markup should render assistant content with markdown support");

  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: {
      getCurrentAdviceDraft: async () => null,
      getAgentMessages: async () => ({ messages: [] }),
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
    "restoreAgentMessages",
    "handleToggleProcess",
    "handleFileSelect",
    "handleChooseImage"
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

  const startedAt = "2026-07-16T09:30:00.000Z";
  const now = new Date("2026-07-16T09:30:05.900Z");
  assert(advisor.formatProcessElapsed(startedAt, "", now) === "5s", "process timer should derive elapsed time from server start time");
  const runningProcess = advisor.normalizeChatProcess({
    status: "running",
    summary: "理解你的需求",
    response_started_at: startedAt,
    steps: []
  }, now);
  assert(runningProcess.summary === "理解你的需求" && runningProcess.elapsed_label === "5s", "running process should retain current summary and elapsed time");
  const finishedProcess = advisor.normalizeChatProcess({
    status: "succeeded",
    summary: "完成回复",
    response_started_at: startedAt,
    finished_at: "2026-07-16T09:30:08.000Z",
    steps: []
  }, now);
  assert(finishedProcess.summary === "查看处理过程" && finishedProcess.elapsed_label === "8s", "finished process should show the disclosure affordance and final elapsed time");
  const failedProcess = advisor.failChatProcess(runningProcess, new Date("2026-07-16T09:30:09.000Z"));
  assert(failedProcess.summary === "查看处理过程" && failedProcess.elapsed_label === "9s", "failed process should stop the timer at its final timestamp");
  const rawAgentPayload = "```json\n" + JSON.stringify({
    assistant_text: "你好，请告诉我你的场景。",
    decision_label: "need_more_info",
    tool_calls: []
  }) + "\n```";
  assert(advisor.extractAssistantText(rawAgentPayload) === "你好，请告诉我你的场景。", "assistant JSON payload should render only assistant_text");

  const page = Object.assign({}, pageConfig, {
    data: JSON.parse(JSON.stringify(pageConfig.data)),
    setData(patch, callback) {
      this.data = Object.assign({}, this.data, patch);
      if (typeof callback === "function") {
        callback();
      }
    }
  });
  let scrollCalls = 0;
  page.scrollToBottom = () => {
    scrollCalls += 1;
  };
  page.data.messages = [{
    id: "assistant-process",
    role: "assistant",
    process: {
      expanded: false,
      status: "succeeded",
      summary: "查看处理过程",
      steps: []
    }
  }];
  page.handleToggleProcess.call(page, { currentTarget: { dataset: { messageId: "assistant-process" } } });
  assert(page.data.messages[0].process.expanded === true, "process toggle should expand the selected message");
  assert(scrollCalls === 0, "expanding process details should keep the current scroll position");
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

  let selectedFile = null;
  global.wx = {
    chooseMedia(options) {
      options.success({
        tempFiles: [{ tempFilePath: "/tmp/chosen-look.jpg", size: 4096 }]
      });
    }
  };
  page.handleFileSelect = async (event) => {
    selectedFile = event.detail.files[0];
  };
  await awaitMaybe(page.handleChooseImage.call(page));
  assert(selectedFile && selectedFile.tempFilePath === "/tmp/chosen-look.jpg", "advisor image picker should pass the selected image to upload handling");

  console.log("advisor page verification passed");
} finally {
  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }
  if (typeof originalWx === "undefined") {
    delete global.wx;
  } else {
    global.wx = originalWx;
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
