package generator

import (
	"context"
	"strings"
)

type RuleReportGenerator struct{}

func NewRuleReportGenerator() *RuleReportGenerator {
	return &RuleReportGenerator{}
}

func (g *RuleReportGenerator) GenerateInitialReport(ctx context.Context, input InitialReportInput) (*InitialReportResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	goals := nonEmptyOrDefault(input.StyleGoals, []string{"干净自然", "更适合日常场景"})
	avoidances := nonEmptyOrDefault(input.Avoidances, []string{"一次性做过大改变", "过度追逐流行导致不自在"})
	scenarios := nonEmptyOrDefault(input.Scenarios, []string{"日常出门"})
	routeName := strings.Join(firstN(goals, 2), " + ")

	hair := StrategyBlock{
		Direction: "保持清爽轮廓，优先让头发服务于脸部精神度和日常打理效率。",
		Steps: []string{
			"先固定一个低维护发型方向，再根据通勤或约会调整蓬松度。",
			"发尾和刘海避免厚重堆积，保留轻微层次。",
		},
		Alternatives: []string{"如果当天时间紧，用低饱和发饰或顺滑发尾替代复杂造型。"},
	}
	makeup := StrategyBlock{
		Direction: "轻量提升气色和精致度，不做强遮盖或夸张风格迁移。",
		Steps: []string{
			"底妆以均匀肤色为主，保留自然肤感。",
			"眉眼和唇色选择低饱和、干净的表达。",
		},
		Alternatives: []string{"素颜场景可只保留眉毛、润唇和局部提亮。"},
	}
	outfit := StrategyBlock{
		Direction: "用清晰比例、低冲突色彩和核心单品复用来形成稳定风格。",
		Steps: []string{
			"上半身保留一个干净视觉中心，下半身用直线条稳定比例。",
			"全身主色控制在两到三类，配饰只做少量点睛。",
		},
		Alternatives: []string{"如果需要更柔和，可以把硬挺外套替换为垂顺针织或开衫。"},
	}

	result := &InitialReportResult{
		Summary:              "你的第一版形象方向建议先围绕" + strings.Join(firstN(goals, 2), "、") + "建立稳定基线，再用场景反馈逐步微调。",
		Routes:               buildRoutes(routeName, scenarios, goals, avoidances, hair, makeup, outfit),
		HairStrategy:         hair,
		MakeupStrategy:       makeup,
		OutfitStrategy:       outfit,
		Avoidances:           avoidances,
		WardrobeCombinations: buildCombinations(input.WardrobeItems, scenarios),
		WardrobeGaps:         buildWardrobeGaps(input.WardrobeItems),
		ActionItems: []string{
			"本周先用 2 件核心单品完成一次真实场景搭配，并记录是否舒服、是否被夸、哪里不自在。",
			"拍一张自然光半身照，用于下次校准发型蓬松度、衣服比例和颜色明度。",
			"先避免一次性购买多件新单品，优先验证现有衣橱的可复用组合。",
		},
		ReferenceStyleLogic: buildReferenceStyleLogic(input.ReferenceStyles),
		PrivacyNote:         "自拍、衣橱照片和审美反馈只用于生成你的个人建议；你可以删除照片、档案和反馈数据。",
		Inferences: []ProfileInferenceInput{
			{Key: "initial_style_direction", Value: goals, Confidence: 0.7},
			{Key: "initial_avoidance_direction", Value: avoidances, Confidence: 0.7},
		},
	}
	return result, nil
}

func buildRoutes(name string, scenarios []string, goals []string, avoidances []string, hair StrategyBlock, makeup StrategyBlock, outfit StrategyBlock) []RouteResult {
	return []RouteResult{
		{
			Name:             name,
			Role:             "primary",
			Weight:           1,
			TargetImpression: firstN(goals, 3),
			SuitableScenes:   firstN(scenarios, 3),
			HairStrategy:     hair,
			MakeupStrategy:   makeup,
			OutfitStrategy:   outfit,
			AvoidPoints:      firstN(avoidances, 4),
			Reason: []string{
				"先用最明确的目标建立可执行基线，后续根据真实反馈更新记忆。",
				"路线强调可迁移的比例、廓形和色彩逻辑，不做相貌比较。",
			},
		},
	}
}

func buildCombinations(items []WardrobeItemInput, scenarios []string) []WardrobeCombination {
	if len(items) == 0 {
		return []WardrobeCombination{
			{
				Title: "基础干净组合",
				Items: []string{"简洁上装", "直线条下装"},
				Scene: scenarios[0],
				Why:   "先用低冲突结构建立稳定比例，便于后续根据照片反馈调整。",
			},
		}
	}
	names := make([]string, 0, min(len(items), 3))
	for _, item := range items {
		if strings.TrimSpace(item.Name) != "" {
			names = append(names, item.Name)
		}
		if len(names) == 3 {
			break
		}
	}
	if len(names) == 0 {
		names = []string{"核心上装", "核心下装"}
	}
	return []WardrobeCombination{
		{
			Title: "现有核心单品复用",
			Items: names,
			Scene: scenarios[0],
			Why:   "先用你已经拥有的单品验证风格方向，可以降低试错成本。",
		},
	}
}

func buildWardrobeGaps(items []WardrobeItemInput) []WardrobeGap {
	hasOuterwear := false
	hasShoe := false
	for _, item := range items {
		category := strings.TrimSpace(item.Category)
		if category == "外套" || strings.Contains(item.Name, "外套") || strings.Contains(item.Name, "西装") {
			hasOuterwear = true
		}
		if category == "鞋" || strings.Contains(item.Name, "鞋") {
			hasShoe = true
		}
	}
	gaps := make([]WardrobeGap, 0, 2)
	if !hasOuterwear {
		gaps = append(gaps, WardrobeGap{
			GapType:     "structure",
			Title:       "一件线条干净的轻外套",
			Description: "优先考虑能覆盖通勤和周末的中性色外套。",
			Reason:      "外套决定第一眼轮廓，能把普通内搭整理成更完整的造型。",
			Priority:    1,
		})
	}
	if !hasShoe {
		gaps = append(gaps, WardrobeGap{
			GapType:     "foundation",
			Title:       "一双简洁低冲突鞋",
			Description: "选择能和裤装、裙装都兼容的基础鞋型。",
			Reason:      "鞋子会影响整体正式度和轻盈利落感，是高频场景的稳定器。",
			Priority:    2,
		})
	}
	if len(gaps) == 0 {
		gaps = append(gaps, WardrobeGap{
			GapType:     "polish",
			Title:       "一个低存在感配饰",
			Description: "用小面积配饰提升完成度，保持颜色和材质克制。",
			Reason:      "当基础单品充足时，配饰能用较低成本改善精致度。",
			Priority:    3,
		})
	}
	return gaps
}

func buildReferenceStyleLogic(refs []string) string {
	if len(refs) == 0 {
		return "参考造型逻辑：先提取比例、廓形、色彩和场景表达，不进行相貌比较；后续根据你的真实反馈保留有效元素。"
	}
	return "参考造型逻辑：可借鉴 " + strings.Join(firstN(refs, 3), "、") + " 的比例、廓形、色彩和发型方向，但只迁移造型方法，不做相貌判断。"
}

func nonEmptyOrDefault(values []string, fallback []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return fallback
	}
	return cleaned
}

func firstN(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
