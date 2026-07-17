const api = require("../../utils/api");

let messageSeq = 0;

Page({
  data: {
    quickScenes: ["通勤怎么穿", "拍照问搭配", "约会建议", "见客户"],
    thinking: false,
    inputValue: "",
    footerStyle: "",
    scrollAnchor: "chat-bottom",
    messages: [
      {
        id: "welcome",
        role: "assistant",
        content: "今天想怎么呈现自己？你可以告诉我场景，也可以拍一件衣服问怎么搭。"
      }
    ]
  },
  async onLoad() {
    this.isUnloaded = false;
    this.startProcessTimer();
    await this.restoreAgentMessages();
    await this.restoreCurrentDraft();
  },
  onUnload() {
    this.isUnloaded = true;
    if (this.activeStreamIdentity && this.activeStreamIdentity.buffer) {
      this.activeStreamIdentity.buffer.clear();
    }
    this.abortActiveRequest("unloaded");
    this.stopProcessTimer();
  },
  async restoreAgentMessages() {
    try {
      const result = await api.getAgentMessages();
      if (this.isUnloaded) return;
      const records = result && Array.isArray(result.messages) ? result.messages : [];
      if (!records.length) {
        return;
      }
      this.setData({
        messages: records.map((message) => normalizeChatMessage(message))
      }, () => {
        this.scrollToBottom();
      });
    } catch (error) {
      if (this.isUnloaded) return;
      // A history request should not prevent a user from starting a new chat.
    }
  },
  startProcessTimer() {
    this.stopProcessTimer();
    this.processTimer = setInterval(() => {
      this.refreshProcessElapsed();
    }, 1000);
  },
  stopProcessTimer() {
    if (this.processTimer) {
      clearInterval(this.processTimer);
      this.processTimer = null;
    }
  },
  refreshProcessElapsed() {
    const now = new Date();
    let changed = false;
    const messages = this.data.messages.map((message) => {
      if (!message.process || !message.process.response_started_at) {
        return message;
      }
      const process = normalizeChatProcess(message.process, now);
      if (process.elapsed_label === message.process.elapsed_label) {
        return message;
      }
      changed = true;
      return Object.assign({}, message, { process });
    });
    if (changed) {
      this.setData({ messages });
    }
  },
  handleToggleProcess(event) {
    const messageID = event.currentTarget.dataset.messageId || "";
    if (!messageID) return;
    const messages = this.data.messages.map((message) => {
      if (message.id !== messageID || !message.process) return message;
      return Object.assign({}, message, {
        process: Object.assign({}, message.process, { expanded: !message.process.expanded })
      });
    });
    this.setData({ messages, scrollAnchor: "" });
  },
  async restoreCurrentDraft() {
    try {
      const draft = await api.getCurrentAdviceDraft();
      if (this.isUnloaded) return;
      if (!draft || !draft.draft_public_id) {
        return;
      }
      const exists = this.data.messages.some((message) => message.draft && message.draft.draft_public_id === draft.draft_public_id);
      if (exists) {
        return;
      }
      this.appendAssistantMessage("继续调整这版草稿也可以。", "", normalizeDraftCard(draft));
    } catch (error) {
      if (this.isUnloaded) return;
      if (error && error.code === "agent.draft_not_found") {
        return;
      }
    }
  },
  handleScene(event) {
    const scene = event.currentTarget.dataset.scene || event.detail.scene;
    if (this.data.thinking) return;
    this.appendUserMessage(scene);
  },
  handleInputChange(event) {
    this.setData({
      inputValue: event.detail.value || ""
    });
  },
  handleKeyboardHeightChange(event) {
    const keyboardBottom = event.detail.height || 0;
    this.setData({
      footerStyle: keyboardBottom ? `padding-bottom: ${keyboardBottom + 16}px` : ""
    });
    this.scrollToBottom();
  },
  handleSend(event) {
    if (this.data.thinking) return;
    const value = (event.detail.value || this.data.inputValue).trim();
    if (!value) return;

    this.setData({
      inputValue: ""
    });
    this.appendUserMessage(value);
  },
  async handleChooseImage() {
    if (this.data.thinking) return;
    const file = await chooseChatImageFile();
    if (!file) return;
    return this.handleFileSelect({ detail: { files: [file] } });
  },
  async handleFileSelect(event) {
    const files = event.detail.files || [];
    if (this.data.thinking) return;
    if (!files.length) return;
    const file = files[0] || {};
    const previewURL = file.url || file.path || file.tempFilePath || "";
    const uploadMessageID = this.appendUserMessage("正在上传照片...", {
      images: previewURL ? [{ url: previewURL }] : [],
      skipSend: true
    });
    this.setData({ thinking: true });
    try {
      const uploaded = await api.uploadFileToQiniu(file, { assetType: "chat_image" });
      const assetRef = {
        asset_public_id: uploaded.asset_public_id || uploaded.file_public_id || "",
        asset_type: "chat_image",
        note: "聊天上传图"
      };
      this.updateMessage(uploadMessageID, (message) => Object.assign({}, message, {
        content: "我想用这张照片提问",
        assetRefs: [assetRef]
      }));
      const assistantMessage = {
        id: nextMessageID("assistant"),
        role: "assistant",
        status: "",
        content: ""
      };
      this.activeAssistantID = assistantMessage.id;
      this.setData({
        messages: this.data.messages.concat(assistantMessage)
      }, () => {
        this.scrollToBottom();
      });
      this.sendToAgent("我想用这张照片提问", assistantMessage.id, [assetRef]);
    } catch (error) {
      this.updateMessage(uploadMessageID, (message) => Object.assign({}, message, {
        status: "error",
        content: error && error.message ? error.message : "照片上传失败"
      }));
      this.setData({ thinking: false });
    }
  },
  handleStop() {
    const assistantID = this.activeAssistantID;
    const identity = this.activeStreamIdentity;
    if (identity && identity.buffer) {
      safeFlushTextDeltaBuffer(identity.buffer);
    }
    this.abortActiveRequest("stopped");
    if (identity && identity.buffer) {
      identity.buffer.clear();
    }
    if (assistantID) {
      this.updateMessage(assistantID, (message) => Object.assign({}, message, {
        status: "stopped",
        content: message.content || "已停止生成。",
        process: stopChatProcess(message.process, new Date())
      }));
    }
    this.setData({ thinking: false });
  },
  handleContinueDraft(event) {
    const draftID = event.currentTarget.dataset.draftId || "";
    if (!draftID || this.data.thinking) return;
    this.setData({
      inputValue: "继续调整这版建议："
    });
  },
  async handleConfirmDraft(event) {
    const draftID = event.currentTarget.dataset.draftId || "";
    if (!draftID || this.data.thinking) return;
    this.setDraftActionState(draftID, true);
    try {
      await api.confirmAdviceDraft(draftID);
      this.updateDraftStatus(draftID, "confirmed");
      this.appendAssistantMessage("已保存为正式建议。");
    } catch (error) {
      this.appendAssistantMessage(error && error.message ? error.message : "保存建议失败", "error");
    } finally {
      this.setDraftActionState(draftID, false);
    }
  },
  async handleDiscardDraft(event) {
    const draftID = event.currentTarget.dataset.draftId || "";
    if (!draftID || this.data.thinking) return;
    this.setDraftActionState(draftID, true);
    try {
      await api.discardAdviceDraft(draftID);
      this.updateDraftStatus(draftID, "discarded");
      this.appendAssistantMessage("这版草稿已丢弃。");
    } catch (error) {
      this.appendAssistantMessage(error && error.message ? error.message : "丢弃草稿失败", "error");
    } finally {
      this.setDraftActionState(draftID, false);
    }
  },
  scrollToBottom() {
    this.setData({ scrollAnchor: "" }, () => {
      this.setData({ scrollAnchor: "chat-bottom" });
    });
  },
  appendUserMessage(content, options) {
    if (!content) return;
    const config = options || {};
    const userMessage = {
      id: nextMessageID("user"),
      role: "user",
      content,
      images: config.images || [],
      assetRefs: config.assetRefs || []
    };
    if (config.skipSend) {
      this.setData({
        messages: this.data.messages.concat(userMessage)
      }, () => {
        this.scrollToBottom();
      });
      return userMessage.id;
    }
    const assistantMessage = {
      id: nextMessageID("assistant"),
      role: "assistant",
      status: "",
      content: ""
    };
    this.activeAssistantID = assistantMessage.id;
    this.setData({
      messages: this.data.messages.concat(userMessage, assistantMessage),
      thinking: true
    }, () => {
      this.scrollToBottom();
    });
    this.sendToAgent(content, assistantMessage.id, config.assetRefs || []);
    return userMessage.id;
  },
  appendAssistantMessage(content, status, draft) {
    const message = {
      id: nextMessageID("assistant"),
      role: "assistant",
      status: status || "",
      content,
      draft
    };
    this.setData({
      messages: this.data.messages.concat(message)
    }, () => {
      this.scrollToBottom();
    });
  },
  async sendToAgent(content, assistantID, assetRefs) {
    if (this.activeStreamIdentity && this.activeStreamIdentity.buffer) {
      safeFlushTextDeltaBuffer(this.activeStreamIdentity.buffer);
    }
    this.abortActiveRequest("superseded");
    const identity = {
      assistantID,
      stream: null,
      stopped: false,
      superseded: false,
      unloaded: false,
      buffer: null
    };
    identity.buffer = createTextDeltaBuffer(Object.assign({}, this.textDeltaBufferOptions || {}, {
      apply: (text) => {
        if (this.isUnloaded || this.activeStreamIdentity !== identity) return;
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: "",
          content: (message.content || "") + text
        }), { suppressScroll: true });
      }
    }));
    this.activeStreamIdentity = identity;
    const stream = api.streamAgentChat({
      text: content,
      assetRefs: assetRefs || []
    }, {
      onStatus: (data) => {
        // Status remains a backwards-compatible server signal. Process events drive the UI.
      },
      onProcess: (data) => {
        if (this.isUnloaded || identity.unloaded) return;
        if (identity.stopped) return;
        if (this.activeStreamIdentity !== identity) return;
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          process: mergeChatProcessEvent(message.process, data, new Date())
        }));
      },
      onDelta: (data) => {
        if (this.isUnloaded || identity.unloaded) return;
        if (identity.stopped) return;
        if (this.activeStreamIdentity !== identity) return;
        identity.buffer.push(data && typeof data.text === "string" ? data.text : "");
      },
      onDraft: (data) => {
        if (this.isUnloaded || identity.unloaded) return;
        if (identity.stopped) return;
        if (this.activeStreamIdentity !== identity) return;
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          draft: normalizeDraftCard(data)
        }));
      },
      onError: (data) => {
        if (this.isUnloaded || identity.unloaded) return;
        safeFlushTextDeltaBuffer(identity.buffer);
        if (this.isUnloaded || identity.unloaded) return;
        if (this.activeStreamIdentity !== identity || identity.stopped) return;
        const text = data && data.message ? data.message : "智能体请求失败";
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: "error",
          content: message.content || text,
          process: failChatProcess(message.process, new Date())
        }));
      },
      onDone: (data) => {
        if (this.isUnloaded || identity.unloaded) return;
        safeFlushTextDeltaBuffer(identity.buffer);
        if (this.isUnloaded || identity.unloaded) return;
        if (this.activeStreamIdentity !== identity || identity.stopped) return;
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: "",
          process: finishChatProcess(message.process, data, new Date())
        }));
      }
    });
    identity.stream = stream;
    this.activeRequest = stream;
    try {
      await stream.promise;
      safeFlushTextDeltaBuffer(identity.buffer);
      if (this.isUnloaded || this.activeStreamIdentity !== identity) return;
      this.setData({ thinking: false }, () => {
        this.scrollToBottom();
      });
    } catch (error) {
      safeFlushTextDeltaBuffer(identity.buffer);
      if (this.isUnloaded || identity.stopped || identity.superseded || this.activeStreamIdentity !== identity) {
        return;
      }
      this.updateMessage(assistantID, (message) => Object.assign({}, message, {
        status: "error",
        content: message.content || (error && error.message ? error.message : "顾问服务请求失败"),
        process: failChatProcess(message.process, new Date())
      }));
      this.setData({ thinking: false });
    } finally {
      safeFlushTextDeltaBuffer(identity.buffer);
      if (this.activeRequest === stream) {
        this.activeRequest = null;
        this.activeAssistantID = "";
        this.activeStreamIdentity = null;
      }
      if (this.stoppingRequest === stream) {
        this.stoppingRequest = null;
      }
    }
  },
  updateMessage(messageID, updater, options) {
    const config = options || {};
    const messages = this.data.messages.map((message) => {
      if (message.id !== messageID) {
        return message;
      }
      return updater(message);
    });
    this.setData({ messages }, config.suppressScroll ? undefined : () => {
      this.scrollToBottom();
    });
  },
  updateDraftStatus(draftID, status) {
    const messages = this.data.messages.map((message) => {
      if (!message.draft || message.draft.draft_public_id !== draftID) {
        return message;
      }
      return Object.assign({}, message, {
        draft: Object.assign({}, message.draft, { status })
      });
    });
    this.setData({ messages });
  },
  setDraftActionState(draftID, loading) {
    const messages = this.data.messages.map((message) => {
      if (!message.draft || message.draft.draft_public_id !== draftID) {
        return message;
      }
      return Object.assign({}, message, {
        draft: Object.assign({}, message.draft, { actionLoading: loading })
      });
    });
    this.setData({ messages });
  },
  abortActiveRequest() {
    const reason = arguments[0] || "stopped";
    const identity = this.activeStreamIdentity;
    if (identity) {
      identity.stopped = reason === "stopped";
      identity.superseded = reason === "superseded";
      identity.unloaded = reason === "unloaded";
    }
    if (this.activeRequest && typeof this.activeRequest.abort === "function") {
      this.stoppingRequest = this.activeRequest;
      this.activeRequest.abort();
    }
  }
});

function nextMessageID(prefix) {
  messageSeq += 1;
  return `${prefix}-${Date.now()}-${messageSeq}`;
}

function chooseChatImageFile() {
  if (typeof wx === "undefined") {
    return Promise.resolve(null);
  }
  if (wx.chooseMedia) {
    return new Promise((resolve) => {
      wx.chooseMedia({
        count: 1,
        mediaType: ["image"],
        sourceType: ["album", "camera"],
        success(result) {
          const files = result && result.tempFiles ? result.tempFiles : [];
          resolve(files[0] || null);
        },
        fail() {
          resolve(null);
        }
      });
    });
  }
  if (wx.chooseImage) {
    return new Promise((resolve) => {
      wx.chooseImage({
        count: 1,
        sourceType: ["album", "camera"],
        success(result) {
          const paths = result && result.tempFilePaths ? result.tempFilePaths : [];
          resolve(paths[0] ? { tempFilePath: paths[0] } : null);
        },
        fail() {
          resolve(null);
        }
      });
    });
  }
  return Promise.resolve(null);
}

function normalizeDraftCard(raw) {
  const draft = raw || {};
  return {
    draft_public_id: draft.draft_public_id || "",
    revision_no: draft.revision_no || 0,
    status: draft.status || "draft",
    scene_label: draft.scene_label || "",
    sections: (draft.sections || []).map(normalizeDraftSection)
  };
}

function normalizeDraftSection(section) {
  const content = section.content_json || {};
  return {
    public_id: section.public_id || "",
    section_type: section.section_type || "",
    label: sectionLabel(section.section_type),
    title: content.title || sectionLabel(section.section_type),
    summary: content.summary || "",
    why_text: content.why_text || "",
    avoid_text: content.avoid_text || "",
    alternative_text: content.alternative_text || ""
  };
}

function sectionLabel(type) {
  if (type === "outfit") return "穿搭";
  if (type === "hair") return "发型";
  if (type === "makeup") return "妆容";
  return "建议";
}

function normalizeChatMessage(raw) {
  const message = raw || {};
  return {
    id: message.public_id || nextMessageID(message.role || "message"),
    role: message.role || "assistant",
    status: message.status === "failed" || message.status === "error" ? "error" : (message.status === "stopped" ? "stopped" : ""),
    content: message.content || "",
    assetRefs: message.asset_refs || [],
    process: message.process ? normalizeChatProcess(message.process, new Date()) : null
  };
}

function createTextDeltaBuffer(options) {
  const config = options || {};
  const delay = typeof config.delay === "number" ? config.delay : 50;
  const apply = config.apply;
  const schedule = config.schedule || ((callback) => setTimeout(callback, delay));
  const cancel = config.cancel || ((timer) => clearTimeout(timer));
  let pending = "";
  let timer = null;

  function flush() {
    if (timer !== null) {
      cancel(timer);
      timer = null;
    }
    if (!pending) return;
    const text = pending;
    pending = "";
    apply(text);
  }

  return {
    push(text) {
      if (typeof text !== "string" || !text) return;
      pending += text;
      if (timer === null) {
        timer = schedule(() => {
          timer = null;
          flush();
        }, delay);
      }
    },
    flush,
    clear() {
      if (timer !== null) {
        cancel(timer);
        timer = null;
      }
      pending = "";
    }
  };
}

function safeFlushTextDeltaBuffer(buffer) {
  try {
    buffer.flush();
  } catch (error) {
    // A transient UI apply failure must not block stream cleanup.
  }
}

function normalizeChatProcess(raw, now) {
  const process = raw || {};
  const status = process.status || "running";
  const finishedAt = process.finished_at || "";
  const running = status === "running";
  return {
    status,
    summary: running ? (process.summary || "理解你的需求") : (status === "stopped" ? "已停止" : "查看处理过程"),
    response_started_at: process.response_started_at || "",
    finished_at: finishedAt,
    elapsed_label: formatProcessElapsed(process.response_started_at, finishedAt, now),
    expanded: Boolean(process.expanded),
    steps: (process.steps || []).map((step) => normalizeChatProcessStep(step))
  };
}

function normalizeChatProcessStep(raw) {
  const step = raw || {};
  return {
    step_no: step.step_no || 0,
    status: step.status || "succeeded",
    summary: step.summary || "整理本轮建议",
    detail: step.detail || "",
    started_at: step.started_at || "",
    finished_at: step.finished_at || ""
  };
}

function mergeChatProcessEvent(existing, event, now) {
  const current = existing || {};
  const source = event || {};
  const steps = (current.steps || []).map(normalizeChatProcessStep);
  const nextStep = normalizeChatProcessStep({
    step_no: source.step_no,
    status: source.status,
    summary: source.summary,
    detail: source.detail,
    started_at: source.response_started_at,
    finished_at: source.finished_at
  });
  if (nextStep.step_no) {
    const index = steps.findIndex((step) => step.step_no === nextStep.step_no);
    if (index >= 0) {
      steps[index] = nextStep;
    } else {
      steps.push(nextStep);
    }
  }
  return normalizeChatProcess({
    status: source.status || current.status || "running",
    summary: source.summary || current.summary,
    response_started_at: source.response_started_at || current.response_started_at,
    finished_at: source.finished_at || current.finished_at,
    expanded: current.expanded,
    steps
  }, now);
}

function finishChatProcess(existing, done, now) {
  const process = existing || {};
  const event = done || {};
  return normalizeChatProcess({
    status: "succeeded",
    response_started_at: event.response_started_at || process.response_started_at,
    finished_at: event.finished_at || process.finished_at || (now || new Date()).toISOString(),
    expanded: process.expanded,
    steps: process.steps || []
  }, now);
}

function failChatProcess(existing, now) {
  const process = existing || {};
  return normalizeChatProcess({
    status: "failed",
    response_started_at: process.response_started_at,
    finished_at: process.finished_at || (now || new Date()).toISOString(),
    expanded: process.expanded,
    steps: process.steps || []
  }, now);
}

function stopChatProcess(existing, now) {
  const process = existing || {};
  return normalizeChatProcess({
    status: "stopped",
    response_started_at: process.response_started_at,
    finished_at: process.finished_at || (now || new Date()).toISOString(),
    expanded: process.expanded,
    steps: process.steps || []
  }, now);
}

function formatProcessElapsed(startedAt, finishedAt, now) {
  const started = Date.parse(startedAt || "");
  if (Number.isNaN(started)) return "";
  const finished = Date.parse(finishedAt || "");
  const end = Number.isNaN(finished) ? (now instanceof Date ? now.getTime() : Date.now()) : finished;
  return `${Math.max(0, Math.floor((end - started) / 1000))}s`;
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeDraftCard,
    normalizeDraftSection,
    sectionLabel,
    normalizeChatMessage,
    normalizeChatProcess,
    formatProcessElapsed,
    failChatProcess,
    stopChatProcess,
    createTextDeltaBuffer
  };
}
