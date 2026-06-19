<script setup lang="ts">
import { NButton, NConfigProvider } from "naive-ui";
import { userNavigation } from "./data/demo";
</script>

<template>
  <NConfigProvider>
    <div class="app-shell">
      <header class="topbar">
        <RouterLink class="brand" to="/">Hestia</RouterLink>
        <nav class="nav">
          <RouterLink v-for="item in userNavigation" :key="item.path" :to="item.path">
            {{ item.label }}
          </RouterLink>
        </nav>
        <NButton type="primary" size="small">打开小程序</NButton>
      </header>
      <main class="page">
        <RouterView />
      </main>
    </div>
  </NConfigProvider>
</template>
