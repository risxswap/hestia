Page({
  data: {
    quickScenes: ["通勤怎么穿", "拍照问搭配", "约会建议", "见客户"],
    thinking: false,
    inputValue: "",
    footerStyle: "",
    chatItems: [],
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
  onLoad() {
    this.syncChatItems();
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
    this.setData({
      thinking: false
    }, () => {
      this.syncChatItems();
    });
  },
  syncChatItems() {
    const chatItems = this.data.messages.map((item) => ({
      id: item.id,
      role: item.role === "user" ? "user" : "assistant",
      placement: item.role === "user" ? "right" : "left",
      name: item.role === "user" ? "你" : "Hestia",
      status: item.status || "",
      content: [
        {
          type: item.type || "text",
          data: item.content || item.text || ""
        }
      ]
    }));

    if (this.data.thinking) {
      chatItems.push({
        id: "thinking",
        role: "assistant",
        placement: "left",
        name: "Hestia",
        status: "pending",
        content: []
      });
    }

    this.setData({ chatItems }, () => {
      this.scrollToBottom();
    });
  },
  scrollToBottom() {
    const chatList = this.selectComponent("#advisor-chat-list");
    if (chatList && typeof chatList.scrollToBottom === "function") {
      chatList.scrollToBottom();
    }
  },
  appendUserMessage(content) {
    if (!content) return;
    const message = {
      id: `user-${Date.now()}`,
      role: "user",
      content
    };
    this.setData({
      messages: this.data.messages.concat(message),
      thinking: true
    }, () => {
      this.syncChatItems();
    });
    this.mockAdvisorReply(content);
  },
  mockAdvisorReply(content) {
    const reply = {
      id: `assistant-${Date.now()}`,
      role: "assistant",
      content: `收到。关于“${content}”，我会先结合你的场景、偏好和已记录禁忌给出可执行建议。`
    };
    setTimeout(() => {
      this.setData({
        messages: this.data.messages.concat(reply),
        thinking: false
      }, () => {
        this.syncChatItems();
      });
    }, 600);
  }
});
