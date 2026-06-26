import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import ReportView from "../views/ReportView.vue";
import RecommendationsView from "../views/RecommendationsView.vue";
import SettingsView from "../views/SettingsView.vue";
import ShareView from "../views/ShareView.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "home", component: HomeView },
    { path: "/report", name: "report", component: ReportView },
    { path: "/recommendations", name: "recommendations", component: RecommendationsView },
    { path: "/settings", name: "settings", component: SettingsView },
    { path: "/share/:type/:token", name: "share", component: ShareView }
  ]
});
