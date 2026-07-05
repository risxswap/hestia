package clothes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hestia/server/internal/infra/llm"
)

var ErrImageRecognizerUnavailable = errors.New("clothes image recognizer unavailable")

type LLMImageRecognizer struct {
	generator llm.Generator
}

func NewLLMImageRecognizer(generator llm.Generator) *LLMImageRecognizer {
	return &LLMImageRecognizer{generator: generator}
}

func (r *LLMImageRecognizer) RecognizeClothesItemImage(ctx context.Context, userID int64, input RecognizeImageInput) (RecognizedItemFields, error) {
	if r == nil || r.generator == nil {
		return RecognizedItemFields{}, ErrImageRecognizerUnavailable
	}
	prompt := clothesImageRecognizePrompt(userID, input.AssetPublicID)
	request := llm.Request{
		UsageKey:     "clothes_image_recognition",
		RequiredCaps: []string{"vision", "json"},
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}
	if strings.TrimSpace(input.ImageURL) != "" {
		request.ImageURLs = []string{strings.TrimSpace(input.ImageURL)}
	}
	response, err := r.generator.Generate(ctx, request)
	if err != nil {
		return RecognizedItemFields{}, err
	}
	var result RecognizedItemFields
	if err := json.Unmarshal([]byte(extractJSONObject(response.Text)), &result); err != nil {
		return RecognizedItemFields{}, fmt.Errorf("parse cloth image recognition result: %w", err)
	}
	return result, nil
}

func clothesImageRecognizePrompt(userID int64, assetPublicID string) string {
	return fmt.Sprintf(`你是个人形象顾问产品里的衣服图片识别模块。
请只根据用户已上传的衣服图片资产 ID 生成可编辑的衣服表单候选字段。
不要输出身材、长相、年龄、肤色评价、医疗或制造焦虑的判断。
不要推荐商品链接、品牌、价格或购买渠道。

用户 ID: %d
图片资产 ID: %s

仅返回 JSON 对象，字段限定为：
{
  "name": "简短单品名称",
  "category": "上装|下装|外套|鞋|包|配饰|运动|家居|其他",
  "color": "主要颜色",
  "silhouette": "廓形",
  "material": "可见或可合理推断的材质",
  "season": "适合季节",
  "scene_tags": ["适用场景"],
  "user_notes": "中性、具体、可行动的搭配提醒",
  "confidence": 0.0
}`, userID, assetPublicID)
}

func extractJSONObject(raw string) string {
	value := strings.TrimSpace(raw)
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start >= 0 && end >= start {
		return value[start : end+1]
	}
	return value
}
