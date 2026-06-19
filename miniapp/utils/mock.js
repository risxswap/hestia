const actionItems = [
  {
    title: "客户场景优先用利落外套",
    reason: "干净肩线能建立更明确的专业感。",
    avoid: "避开太软塌的针织开衫。"
  },
  {
    title: "发型减少厚重感",
    reason: "保留层次和额头呼吸感。",
    avoid: "不要把刘海和发尾都处理得过厚。"
  }
];

const todayRecommendation = {
  context: "周五 22° / 见客户后的晚餐",
  label: "今日推荐",
  title: "浅外套 + 直筒裤，轻松但有精神",
  summary: "适合今天从客户场景切到晚餐场景，保留干净线条，也不会显得太用力。",
  visualTitle: "柔和浅色层次",
  visualMeta: "用你常穿的浅外套做主角",
  primaryAction: "照这个穿",
  secondaryAction: "换个场景"
};

const todayPlanSections = [
  {
    title: "为什么适合今天",
    body: "浅色短外套能提亮上半身，直筒裤保持利落感，适合需要亲和但不松散的场合。"
  },
  {
    title: "发型方向",
    body: "保留额头附近的呼吸感，发尾不要压得太厚，让整体更轻。"
  },
  {
    title: "今天不优先",
    body: "不优先软塌针织和过甜的裙装。如果想更温柔，可以换成有筋骨感的浅色开衫。"
  },
  {
    title: "可替换单品",
    body: "没有浅外套时，用米白衬衫外搭薄马甲；鞋子优先低跟单鞋或干净乐福鞋。"
  }
];

const feedbackOptions = [
  "照这个穿",
  "不喜欢",
  "换正式一点",
  "换轻松一点",
  "我实际这样穿了"
];

module.exports = {
  actionItems,
  todayRecommendation,
  todayPlanSections,
  feedbackOptions
};
