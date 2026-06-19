import { createRouter, createWebHistory } from "vue-router";
import DashboardView from "../views/DashboardView.vue";
import StyleLibraryView from "../views/StyleLibraryView.vue";
import AiConfigView from "../views/AiConfigView.vue";
import JobsView from "../views/JobsView.vue";
import UsersView from "../views/UsersView.vue";
import SystemView from "../views/SystemView.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", name: "dashboard", component: DashboardView },
    { path: "/style-library", name: "style-library", component: StyleLibraryView },
    { path: "/ai-config", name: "ai-config", component: AiConfigView },
    { path: "/jobs", name: "jobs", component: JobsView },
    { path: "/users", name: "users", component: UsersView },
    { path: "/system", name: "system", component: SystemView }
  ]
});
