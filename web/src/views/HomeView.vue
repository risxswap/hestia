<script setup lang="ts">
import { NButton, NTag } from "naive-ui";
import { demoActionItems } from "../data/demo";
</script>

<template>
  <div class="home-grid">
    <section class="hero panel">
      <p class="eyebrow">当前路线</p>
      <h1>利落轻熟 · 低饱和 · 不甜腻</h1>
      <p class="summary">适合通勤、客户会面和轻正式社交。Web 端用于复盘，主要提问仍回到小程序完成。</p>
      <div class="actions">
        <NButton type="primary">查看完整报告</NButton>
        <NButton secondary>补充反馈</NButton>
      </div>
    </section>

    <aside class="panel side">
      <h2 class="section-title">待反馈</h2>
      <p>上次“见客户”建议还没有记录实际效果。</p>
      <NTag type="warning" :bordered="false">会影响后续记忆更新</NTag>
    </aside>

    <section class="panel action-list">
      <h2 class="section-title">你的下一步</h2>
      <article v-for="item in demoActionItems" :key="item.id" class="action-item">
        <strong>{{ item.title }}</strong>
        <p>{{ item.reason }}</p>
        <small>{{ item.avoid }}</small>
      </article>
    </section>
  </div>
</template>

<style scoped>
.home-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(280px, 0.8fr);
  gap: 20px;
}

.hero {
  padding: clamp(28px, 5vw, 56px);
}

.eyebrow {
  margin: 0 0 10px;
  color: var(--color-accent);
  font-weight: 700;
}

h1 {
  max-width: 760px;
  margin: 0;
  font-size: clamp(34px, 5vw, 64px);
  line-height: 1.05;
  letter-spacing: 0;
}

.summary {
  max-width: 640px;
  color: var(--color-muted);
  font-size: 17px;
  line-height: 1.8;
}

.actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.side,
.action-list {
  padding: 24px;
}

.action-list {
  grid-column: 1 / -1;
}

.action-item {
  padding: 18px 0;
  border-top: 1px solid var(--color-line);
}

.action-item:first-of-type {
  border-top: 0;
}

.action-item p {
  margin: 8px 0;
  color: var(--color-muted);
}

.action-item small {
  color: var(--color-accent);
}

@media (max-width: 880px) {
  .home-grid {
    grid-template-columns: 1fr;
  }
}
</style>
