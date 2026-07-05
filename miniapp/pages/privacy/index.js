function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({ title, icon: icon || "none" });
  }
}

const privacyPageConfig = {
  data: {
    clearing: false
  },

  handleClearLocalSession() {
    this.setData({ clearing: true });
    if (typeof wx !== "undefined" && wx.removeStorageSync) {
      ["user_token", "token", "user_public_id", "onboarding_status"].forEach((key) => {
        wx.removeStorageSync(key);
      });
    }
    this.setData({ clearing: false });
    showToast("已清除本地登录");
  }
};

if (typeof Page === "function") {
  Page(privacyPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    privacyPageConfig
  };
}
