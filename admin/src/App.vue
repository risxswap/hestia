<script setup lang="ts">
import { adminNavigation } from "./data/navigation";
</script>

<template>
  <ElContainer class="admin-shell">
    <ElAside width="232px" class="sidebar">
      <div class="brand">Hestia Admin</div>
      <ElMenu router :default-active="$route.path">
        <ElMenuItem v-for="item in adminNavigation" :key="item.path" :index="item.path">
          {{ item.label }}
        </ElMenuItem>
      </ElMenu>
    </ElAside>
    <ElContainer>
      <ElHeader class="header">
        <span>运营管理台</span>
        <ElTag type="success">MVP</ElTag>
      </ElHeader>
      <ElMain>
        <RouterView />
      </ElMain>
    </ElContainer>
  </ElContainer>
</template>
