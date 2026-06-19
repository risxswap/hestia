<template>
  <section class="page-card">
    <div class="toolbar inner">
      <h1>风格库</h1>
      <ElButton type="primary">新增参考对象</ElButton>
    </div>
    <ElTable :data="rows" border>
      <ElTableColumn prop="name" label="参考对象" />
      <ElTableColumn prop="type" label="类型" />
      <ElTableColumn prop="status" label="状态" />
      <ElTableColumn prop="updatedAt" label="更新时间" />
    </ElTable>
  </section>
</template>

<script setup lang="ts">
const rows = [
  { name: "通勤利落样本", type: "内部样本", status: "草稿", updatedAt: "2026-06-03" }
];
</script>

<style scoped>
.inner {
  padding: 16px;
}
</style>
