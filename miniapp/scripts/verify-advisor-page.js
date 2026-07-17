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
const streamCalls = [];
let apiMock = null;

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
  assert(!advisorWXML.includes("'你' : 'Hestia'"), "chat messages should not render sender names");
  assert(!advisorWXML.includes('class="chat-message__meta"'), "chat messages should not render sender metadata");
  assert(advisorJSON.includes('"t-chat-markdown"'), "advisor page should register TDesign markdown rendering");
  assert(advisorWXML.includes("<t-chat-markdown"), "advisor markup should render assistant content with markdown support");

  apiMock = {
      getCurrentAdviceDraft: async () => null,
      getAgentMessages: async () => ({ messages: [] }),
      uploadFileToQiniu: async (file, options) => {
        apiCalls.push({ name: "uploadFileToQiniu", file, options });
        return { asset_public_id: "ast_chat_photo" };
      },
      streamAgentChat: (payload, handlers) => {
        apiCalls.push({ name: "streamAgentChat", payload });
        streamCalls.push({ payload, handlers });
        return {
          promise: new Promise(() => {}),
          abort() {}
        };
      },
      confirmAdviceDraft: async () => ({}),
      discardAdviceDraft: async () => ({})
  };
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: apiMock
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
  assert(typeof advisor.createTextDeltaBuffer === "function", "advisor should export createTextDeltaBuffer for verification");
  assert(typeof advisor.stopChatProcess === "function", "advisor should export stopChatProcess for verification");
  const stoppedHistory = advisor.normalizeChatMessage({
    public_id: "msg-stopped",
    role: "assistant",
    status: "stopped",
    content: "保留的正文",
    process: { status: "stopped", summary: "旧摘要", response_started_at: startedAt, steps: [] }
  });
  assert(stoppedHistory.status === "stopped", "historical stopped message should preserve stopped status");
  assert(stoppedHistory.content === "保留的正文", "historical assistant should use natural text directly");
  assert(stoppedHistory.process.summary === "已停止", "historical stopped process should show stopped summary");
  assert(advisor.normalizeChatMessage({ role: "assistant", status: "failed" }).status === "error", "historical failed status should normalize to error");
  assert(advisor.normalizeChatMessage({ role: "assistant", status: "error" }).status === "error", "internal error status should remain error when normalized");

  const timers = [];
  const applied = [];
  const buffer = advisor.createTextDeltaBuffer({
    apply(text) { applied.push(text); },
    schedule(callback) { timers.push(callback); return callback; },
    cancel(timer) { const index = timers.indexOf(timer); if (index >= 0) timers.splice(index, 1); }
  });
  buffer.push("第一段 ");
  buffer.push("\n第二段");
  assert(applied.length === 0, "consecutive delta pushes should not apply before the timer runs");
  assert(timers.length === 1, "consecutive delta pushes should schedule only one timer");
  timers.shift()();
  assert(applied.join("") === "第一段 \n第二段", "buffer should preserve whitespace and apply all pending text once");
  buffer.push("待完成");
  buffer.flush();
  assert(applied.join("") === "第一段 \n第二段待完成", "flush should synchronously apply pending text");
  const applyCount = applied.length;
  buffer.flush();
  assert(applied.length === applyCount, "flushing an empty buffer should have no side effect");
  buffer.push("应丢弃");
  buffer.clear();
  assert(timers.length === 0, "clear should cancel the pending timer");
  assert(applied.join("") === "第一段 \n第二段待完成", "clear should discard pending text");

  const throwingTimers = [];
  let throwingApply = true;
  const throwingBuffer = advisor.createTextDeltaBuffer({
    apply() { if (throwingApply) throw new Error("setData failed"); },
    schedule(callback) { throwingTimers.push(callback); return callback; },
    cancel(timer) { const index = throwingTimers.indexOf(timer); if (index >= 0) throwingTimers.splice(index, 1); }
  });
  throwingBuffer.push("第一次");
  try { throwingBuffer.flush(); } catch (error) {}
  assert(throwingTimers.length === 0, "apply failure should leave no pending timer");
  throwingApply = false;
  throwingBuffer.push("第二次");
  assert(throwingTimers.length === 1, "buffer should schedule again after apply failure");
  throwingBuffer.flush();

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

  const streamedAssistant = page.data.messages.find((message) => message.role === "assistant" && message.content === "");
  const photoStream = streamCalls[0];
  photoStream.handlers.onDelta({ text: "米白 " });
  photoStream.handlers.onDelta({ text: "衬衫\n配直筒裤" });
  assert(streamedAssistant.content === "", "two deltas should not update the page before the buffer timer");
  photoStream.handlers.onDone({});
  const completedAssistant = page.data.messages.find((message) => message.id === streamedAssistant.id);
  assert(completedAssistant.content === "米白 衬衫\n配直筒裤", "done should synchronously flush deltas without duplication");

  const stoppedPage = createPage(pageConfig, [{ id: "assistant-stop", role: "assistant", status: "", content: "" }]);
  stoppedPage.activeAssistantID = "assistant-stop";
  let stoppedScheduled = 0;
  stoppedPage.textDeltaBufferOptions = {
    schedule(callback) { stoppedScheduled += 1; return callback; },
    cancel() {}
  };
  let stopHandlers = null;
  let rejectStopped = null;
  apiMock.streamAgentChat = (payload, handlers) => {
    stopHandlers = handlers;
    return {
      promise: new Promise((resolve, reject) => { rejectStopped = reject; }),
      abort() { rejectStopped(new Error("request:fail abort")); }
    };
  };
  const stoppedSend = stoppedPage.sendToAgent.call(stoppedPage, "停止测试", "assistant-stop", []);
  stopHandlers.onDelta({ text: "已经生成的正文 " });
  stopHandlers.onDelta({ text: "不能丢" });
  stoppedPage.handleStop.call(stoppedPage);
  await stoppedSend;
  const stoppedMessage = stoppedPage.data.messages[0];
  assert(stoppedMessage.content === "已经生成的正文 不能丢", "stop should flush and preserve pending delta content");
  assert(stoppedMessage.status === "stopped", "stop should keep stopped status after abort rejection");
  assert(stoppedMessage.process.status === "stopped" && stoppedMessage.process.summary === "已停止", "stop should use a stopped process summary");
  const stoppedContent = stoppedMessage.content;
  stopHandlers.onProcess({ step_no: 9, status: "running", summary: "停止后过程" });
  stopHandlers.onDelta({ text: "停止后晚到" });
  stopHandlers.onDraft({ draft_public_id: "drf_after_stop", sections: [] });
  await wait(70);
  const stoppedAfterLateEvents = stoppedPage.data.messages[0];
  assert(stoppedAfterLateEvents.content === stoppedContent && stoppedAfterLateEvents.status === "stopped", "late events after stop must not change content or status");
  assert(!stoppedAfterLateEvents.draft, "late draft after stop must be ignored");
  assert(stoppedScheduled === 1, "late delta after stop must not schedule another buffer timer");

  const rejectedPage = createPage(pageConfig, [{ id: "assistant-reject", role: "assistant", status: "", content: "" }]);
  let rejectHandlers = null;
  let rejectPartial = null;
  apiMock.streamAgentChat = (payload, handlers) => {
    rejectHandlers = handlers;
    return {
      promise: new Promise((resolve, reject) => { rejectPartial = reject; }),
      abort() {}
    };
  };
  const rejectedSend = rejectedPage.sendToAgent.call(rejectedPage, "失败测试", "assistant-reject", []);
  rejectHandlers.onDelta({ text: "已经收到的部分正文" });
  rejectPartial(new Error("底层连接错误"));
  await rejectedSend;
  const rejectedMessage = rejectedPage.data.messages[0];
  assert(rejectedMessage.content === "已经收到的部分正文", "promise rejection should preserve flushed partial content");
  assert(rejectedMessage.status === "error" && rejectedMessage.process.status === "failed", "promise rejection should mark the partial response failed");

  const unloadedPage = createPage(pageConfig, [{ id: "assistant-unload", role: "assistant", status: "", content: "" }]);
  let unloadHandlers = null;
  apiMock.streamAgentChat = (payload, handlers) => {
    unloadHandlers = handlers;
    return { promise: new Promise(() => {}), abort() {} };
  };
  let unloadScheduled = 0;
  let unloadCancelled = 0;
  unloadedPage.textDeltaBufferOptions = {
    schedule(callback) {
      unloadScheduled += 1;
      return callback;
    },
    cancel() {
      unloadCancelled += 1;
    }
  };
  unloadedPage.sendToAgent.call(unloadedPage, "卸载测试", "assistant-unload", []);
  unloadHandlers.onDelta({ text: "卸载前待清理" });
  assert(unloadScheduled === 1, "pre-unload delta should schedule one buffer timer");
  const unloadSetDataCount = unloadedPage.setDataCalls;
  unloadedPage.onUnload.call(unloadedPage);
  assert(unloadCancelled === 1, "unload should cancel the pending buffer timer");
  unloadHandlers.onProcess({ step_no: 1, status: "running", summary: "卸载后过程" });
  unloadHandlers.onDelta({ text: "卸载后晚到正文" });
  unloadHandlers.onDraft({ draft_public_id: "drf_late", sections: [] });
  unloadHandlers.onError({ message: "卸载后错误" });
  unloadHandlers.onDone({});
  await wait(70);
  assert(unloadedPage.setDataCalls === unloadSetDataCount, "late error/done and cleared timer must not setData after unload");
  assert(unloadScheduled === 1, "late delta after unload must not schedule another buffer timer");
  assert(unloadedPage.data.messages[0].content === "", "unload should clear pending delta without applying it");

  let batchTimer = null;
  const batchPage = createPage(pageConfig, [{ id: "assistant-batch", role: "assistant", status: "", content: "" }]);
  batchPage.textDeltaBufferOptions = {
    schedule(callback) { batchTimer = callback; return callback; },
    cancel() { batchTimer = null; }
  };
  let batchHandlers = null;
  apiMock.streamAgentChat = (payload, handlers) => {
    batchHandlers = handlers;
    return { promise: new Promise(() => {}), abort() {} };
  };
  batchPage.sendToAgent.call(batchPage, "批量测试", "assistant-batch", []);
  batchHandlers.onDelta({ text: "第一" });
  batchHandlers.onDelta({ text: "第二" });
  const batchSetDataBefore = batchPage.setDataCalls || 0;
  batchTimer();
  assert((batchPage.setDataCalls || 0) - batchSetDataBefore === 1, "one delta batch should perform at most one messages setData");
  assert((batchPage.scrollCalls || 0) === 0, "delta batch should not trigger scroll-anchor setData");

  let resolveFaulty = null;
  const faultyPage = createPage(pageConfig, [{ id: "assistant-faulty", role: "assistant", status: "", content: "" }]);
  apiMock.streamAgentChat = (payload, handlers) => ({
    promise: new Promise((resolve) => { resolveFaulty = resolve; }),
    abort() {}
  });
  const originalFaultySetData = faultyPage.setData;
  faultyPage.setData = function setDataWithFailure(patch, callback) {
    if (patch.messages) throw new Error("setData failed");
    return originalFaultySetData.call(this, patch, callback);
  };
  const faultySend = faultyPage.sendToAgent.call(faultyPage, "异常清理", "assistant-faulty", []);
  faultyPage.activeStreamIdentity.buffer.push("触发异常");
  resolveFaulty([]);
  await faultySend;
  assert(faultyPage.activeRequest === null && faultyPage.activeStreamIdentity === null, "UI apply failure must not prevent active stream cleanup");

  let resolveHistory = null;
  let resolveDraft = null;
  apiMock.getAgentMessages = () => new Promise((resolve) => { resolveHistory = resolve; });
  apiMock.getCurrentAdviceDraft = () => new Promise((resolve) => { resolveDraft = resolve; });
  const restoreHistoryPage = createPage(pageConfig, []);
  const historyRestore = restoreHistoryPage.restoreAgentMessages.call(restoreHistoryPage);
  restoreHistoryPage.onUnload.call(restoreHistoryPage);
  const historySetDataCount = restoreHistoryPage.setDataCalls || 0;
  resolveHistory({ messages: [{ public_id: "late-history", role: "assistant", content: "晚到历史" }] });
  await historyRestore;
  assert((restoreHistoryPage.setDataCalls || 0) === historySetDataCount, "history resolving after unload must not setData");
  const restoreDraftPage = createPage(pageConfig, []);
  const draftRestore = restoreDraftPage.restoreCurrentDraft.call(restoreDraftPage);
  restoreDraftPage.onUnload.call(restoreDraftPage);
  const draftSetDataCount = restoreDraftPage.setDataCalls || 0;
  resolveDraft({ draft_public_id: "late-draft", sections: [] });
  await draftRestore;
  assert((restoreDraftPage.setDataCalls || 0) === draftSetDataCount, "draft resolving after unload must not append a message");

  const racePage = createPage(pageConfig, [
    { id: "assistant-old", role: "assistant", status: "", content: "" },
    { id: "assistant-new", role: "assistant", status: "", content: "" }
  ]);
  const races = [];
  apiMock.streamAgentChat = (payload, handlers) => {
    let resolveStream;
    let rejectStream;
    const call = { handlers };
    call.promise = new Promise((resolve, reject) => { resolveStream = resolve; rejectStream = reject; });
    call.resolve = resolveStream;
    call.abort = () => rejectStream(new Error("request:fail abort"));
    races.push(call);
    return { promise: call.promise, abort: call.abort };
  };
  const oldSend = racePage.sendToAgent.call(racePage, "旧请求", "assistant-old", []);
  races[0].handlers.onDelta({ text: "旧请求已到" });
  const newSend = racePage.sendToAgent.call(racePage, "新请求", "assistant-new", []);
  races[0].handlers.onDelta({ text: "旧请求晚到" });
  races[1].handlers.onDelta({ text: "新回复" });
  races[1].handlers.onDone({});
  races[1].resolve([]);
  await Promise.all([oldSend, newSend]);
  assert(racePage.data.messages[0].content === "旧请求已到", "superseding should flush received text but ignore late deltas");
  assert(racePage.data.messages[1].content === "新回复", "current request delta should update only its assistant message");

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
  page.data.thinking = false;
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

function createPage(config, messages) {
  return Object.assign({}, config, {
    data: Object.assign({}, JSON.parse(JSON.stringify(config.data)), {
      messages: JSON.parse(JSON.stringify(messages || [])),
      thinking: true
    }),
    setData(patch, callback) {
      this.setDataCalls = (this.setDataCalls || 0) + 1;
      this.data = Object.assign({}, this.data, patch);
      if (typeof callback === "function") callback();
    },
    scrollToBottom() {
      this.scrollCalls = (this.scrollCalls || 0) + 1;
    }
  });
}

function wait(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
