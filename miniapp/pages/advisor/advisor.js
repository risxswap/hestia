const api = require("../../utils/api");

let messageSeq = 0;

Page({
  data: {
    quickScenes: ["通勤怎么穿", "拍照问搭配", "约会建议", "见客户"],
    thinking: false,
    inputValue: "",
    footerStyle: "",
    scrollAnchor: "chat-bottom",
    tdesignConfig: {
      chatSender: {
        sendText: "停止",
        stopText: "发送"
      }
    },
    renderPresets: [
      {
        name: "upload",
        presets: ["uploadCamera", "uploadImage"],
        status: ""
      },
      {
        name: "send",
        type: "text"
      }
    ],
    textareaProps: {
      autosize: {
        minHeight: 42,
        maxHeight: 120
      }
    },
    messages: [
      {
        id: "welcome",
        role: "assistant",
        content: "今天想怎么呈现自己？你可以告诉我场景，也可以拍一件衣服问怎么搭。"
      }
    ]
  },
  onUnload() {
    this.abortActiveRequest();
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
  handleFileSelect(event) {
    const files = event.detail.files || [];
    if (this.data.thinking) return;
    if (!files.length) return;
    this.appendUserMessage("我想用这张照片提问");
  },
  handleStop() {
    this.abortActiveRequest();
    const assistantID = this.activeAssistantID;
    if (assistantID) {
      this.updateMessage(assistantID, (message) => Object.assign({}, message, {
        status: "",
        content: message.content || "已停止生成。"
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
  appendUserMessage(content) {
    if (!content) return;
    const userMessage = {
      id: nextMessageID("user"),
      role: "user",
      content
    };
    const assistantMessage = {
      id: nextMessageID("assistant"),
      role: "assistant",
      status: "pending",
      content: "正在理解你的需求..."
    };
    this.activeAssistantID = assistantMessage.id;
    this.setData({
      messages: this.data.messages.concat(userMessage, assistantMessage),
      thinking: true
    }, () => {
      this.scrollToBottom();
    });
    this.sendToAgent(content, assistantMessage.id);
  },
  appendAssistantMessage(content, status) {
    const message = {
      id: nextMessageID("assistant"),
      role: "assistant",
      status: status || "",
      content
    };
    this.setData({
      messages: this.data.messages.concat(message)
    }, () => {
      this.scrollToBottom();
    });
  },
  async sendToAgent(content, assistantID) {
    this.abortActiveRequest();
    let receivedMessage = false;
    const stream = api.streamAgentChat(content, {
      onStatus: (data) => {
        const text = data && data.text ? data.text : "正在准备建议...";
        if (!receivedMessage) {
          this.updateMessage(assistantID, (message) => Object.assign({}, message, {
            status: "pending",
            content: text
          }));
        }
      },
      onMessage: (data) => {
        receivedMessage = true;
        const text = data && data.text ? data.text : "";
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: "pending",
          content: text || message.content
        }));
      },
      onDraft: (data) => {
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          draft: normalizeDraftCard(data)
        }));
      },
      onError: (data) => {
        const text = data && data.message ? data.message : "智能体请求失败";
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: "error",
          content: text
        }));
      },
      onDone: () => {
        this.updateMessage(assistantID, (message) => Object.assign({}, message, {
          status: ""
        }));
      }
    });
    this.activeRequest = stream;
    try {
      await stream.promise;
      this.setData({ thinking: false }, () => {
        this.scrollToBottom();
      });
    } catch (error) {
      if (this.stoppingRequest) {
        return;
      }
      this.updateMessage(assistantID, (message) => Object.assign({}, message, {
        status: "error",
        content: error && error.message ? error.message : "顾问服务请求失败"
      }));
      this.setData({ thinking: false });
    } finally {
      if (this.activeRequest === stream) {
        this.activeRequest = null;
        this.activeAssistantID = "";
      }
      this.stoppingRequest = false;
    }
  },
  updateMessage(messageID, updater) {
    const messages = this.data.messages.map((message) => {
      if (message.id !== messageID) {
        return message;
      }
      return updater(message);
    });
    this.setData({ messages }, () => {
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
    if (this.activeRequest && typeof this.activeRequest.abort === "function") {
      this.stoppingRequest = true;
      this.activeRequest.abort();
    }
  }
});

function nextMessageID(prefix) {
  messageSeq += 1;
  return `${prefix}-${Date.now()}-${messageSeq}`;
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

if (typeof module !== "undefined") {
  module.exports = {
    normalizeDraftCard,
    normalizeDraftSection,
    sectionLabel
  };
}
