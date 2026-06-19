export interface ActionItem {
  id: string;
  title: string;
  reason: string;
  avoid: string;
}

export const userNavigation = [
  { label: "首页", path: "/" },
  { label: "报告", path: "/report" },
  { label: "建议", path: "/recommendations" },
  { label: "设置", path: "/settings" }
] as const;

export const demoActionItems: ActionItem[] = [
  {
    id: "outerwear",
    title: "客户场景优先用利落外套",
    reason: "干净肩线能建立更明确的专业感。",
    avoid: "避开太软塌的针织开衫。"
  },
  {
    id: "hair",
    title: "发型减少厚重感",
    reason: "保留层次和额头呼吸感，整体会更轻。",
    avoid: "不要把发尾和刘海都处理得过厚。"
  },
  {
    id: "gap",
    title: "补一件浅色短外套",
    reason: "长度到胯上、线条干净，适合通勤和轻正式场景。",
    avoid: "暂不建议买装饰过多或轮廓松散的款式。"
  }
];
