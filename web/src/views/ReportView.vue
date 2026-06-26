<script setup lang="ts">
import { demoActionItems } from "../data/demo";
</script>

<template>
  <section class="panel report">
    <p class="meta">报告更新时间：2026-06-03</p>
    <h1>个人形象报告</h1>
    <div class="items">
      <article v-for="item in demoActionItems" :key="item.id">
        <h2>{{ item.title }}</h2>
        <p>{{ item.reason }}</p>
        <small>{{ item.avoid }}</small>
      </article>
    </div>
  </section>
</template>

<style scoped>
.report {
  padding: clamp(24px, 5vw, 48px);
}

.meta {
  color: var(--color-muted);
}

h1 {
  margin: 0 0 24px;
  font-size: 40px;
  letter-spacing: 0;
}

article {
  padding: 20px 0;
  border-top: 1px solid var(--color-line);
}

h2 {
  margin: 0 0 8px;
}

p {
  color: var(--color-muted);
}

small {
  color: var(--color-accent);
}
</style>
