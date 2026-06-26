const api = require("../../utils/api");

const profilePageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    userPublicID: "",
    onboardingStatus: "",
    tokenReady: false,
    items: ["偏好和禁忌", "反馈历史", "隐私与数据"]
  },

  onLoad() {
    return this.loadProfile();
  },

  async loadProfile() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const session = await api.ensureDevSession();
      this.setData({
        loading: false,
        userPublicID: session.user_public_id || "",
        onboardingStatus: session.onboarding_status || "",
        tokenReady: Boolean(session.token)
      });
    } catch (error) {
      this.setData({
        loading: false,
        tokenReady: false,
        errorMessage: error && error.message ? error.message : "登录状态读取失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(profilePageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    profilePageConfig
  };
}
