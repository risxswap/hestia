<template>
  <div>
    <div class="toolbar">
      <h1>工作台</h1>
      <ElButton type="primary">刷新待办</ElButton>
    </div>
    <ElRow :gutter="16">
      <ElCol :span="6">
        <ElCard shadow="never"><ElStatistic title="今日待办" :value="12" /></ElCard>
      </ElCol>
      <ElCol :span="6">
        <ElCard shadow="never"><ElStatistic title="待发布样本" :value="5" /></ElCard>
      </ElCol>
      <ElCol :span="6">
        <ElCard shadow="never"><ElStatistic title="失败任务" :value="3" /></ElCard>
      </ElCol>
      <ElCol :span="6">
        <ElCard shadow="never"><ElStatistic title="删除请求" :value="1" /></ElCard>
      </ElCol>
    </ElRow>
  </div>
</template>
