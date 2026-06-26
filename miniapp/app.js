const api = require("./utils/api");

async function routeFirstEntry() {
  if (wx.getStorageSync("onboarding_skip")) {
    return;
  }

  const session = await api.ensureDevSession();
  if (session && session.onboarding_status === "completed") {
    return;
  }

  const report = await api.getLatestReport();
  if (report) {
    return;
  }

  const draft = await api.getOnboardingDraft();
  if (!draft || draft.status !== "submitted") {
    wx.reLaunch({ url: "/pages/onboarding/onboarding" });
  }
}

App({
  globalData: {
    apiBaseUrl: "http://127.0.0.1:8080",
    privacyVersion: "2026-06-03"
  },

  onLaunch() {
    return routeFirstEntry().catch(() => {});
  }
});
