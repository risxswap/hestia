Page({
  data: {
    quickScenes: ["通勤怎么穿", "拍照问搭配", "约会建议", "见客户"],
    messages: [
      {
        role: "advisor",
        text: "今天想怎么呈现自己？你可以告诉我场景，也可以拍一件衣服问怎么搭。"
      }
    ]
  },
  handleSceneTap(event) {
    const scene = event.currentTarget.dataset.scene;
    this.setData({
      messages: this.data.messages.concat({
        role: "user",
        text: scene
      })
    });
  }
});
