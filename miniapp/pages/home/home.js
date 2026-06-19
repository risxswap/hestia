const {
  feedbackOptions,
  todayPlanSections,
  todayRecommendation
} = require("../../utils/mock");

Page({
  data: {
    updatedAt: "2026-06-19",
    feedbackOptions,
    todayPlanSections,
    todayRecommendation,
    memoryToast: ""
  },

  handlePrimaryAction() {
    this.setData({
      memoryToast: "已记住：你今天更喜欢利落但不强势的感觉。"
    });
  },

  handleSceneChange() {
    this.setData({
      memoryToast: "可以告诉我新场景，我会换一套更贴近的建议。"
    });
  },

  handleFeedback(event) {
    const value = event.currentTarget.dataset.value;
    this.setData({
      memoryToast: `已收到：${value}`
    });
  }
});
