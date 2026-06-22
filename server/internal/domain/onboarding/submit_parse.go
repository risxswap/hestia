package onboarding

import (
	"encoding/json"
	"strings"

	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/generator"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/wardrobe"
)

type parsedSubmitDraft struct {
	basic             map[string]any
	profile           profile.OnboardingInput
	assets            []asset.Input
	wardrobe          []wardrobe.Input
	generatorWardrobe []generator.WardrobeItemInput
	referenceStyles   []string
}

func parseSubmitDraft(data DraftData) (parsedSubmitDraft, error) {
	if len(data) == 0 {
		return parsedSubmitDraft{}, ValidationError{Field: "draft", Message: "required"}
	}
	var basic basicDraft
	if err := decodeRequired(data, "basic", &basic); err != nil {
		return parsedSubmitDraft{}, err
	}
	var style styleGoalDraft
	if err := decodeRequired(data, "style_goal", &style); err != nil {
		return parsedSubmitDraft{}, err
	}
	var wardrobeData wardrobeDraft
	if err := decodeRequired(data, "wardrobe", &wardrobeData); err != nil {
		return parsedSubmitDraft{}, err
	}
	if len(style.Goals) == 0 {
		return parsedSubmitDraft{}, ValidationError{Field: "style_goal.goals", Message: "required"}
	}
	if len(wardrobeData.Items) == 0 {
		return parsedSubmitDraft{}, ValidationError{Field: "wardrobe.items", Message: "required"}
	}

	basicMap := map[string]any{}
	_ = json.Unmarshal(data["basic"], &basicMap)
	scenarios := cleanStrings(style.Scenarios)
	if len(scenarios) == 0 {
		scenarios = []string{"日常出门"}
	}
	wardrobeInputs := make([]wardrobe.Input, 0, len(wardrobeData.Items))
	generatorInputs := make([]generator.WardrobeItemInput, 0, len(wardrobeData.Items))
	for _, item := range wardrobeData.Items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		wardrobeInputs = append(wardrobeInputs, wardrobe.Input{
			Name:       item.Name,
			Category:   item.Category,
			Color:      item.Color,
			Silhouette: item.Silhouette,
			Material:   item.Material,
			Season:     item.Season,
			Notes:      item.Notes,
		})
		generatorInputs = append(generatorInputs, generator.WardrobeItemInput{
			Name:       item.Name,
			Category:   item.Category,
			Color:      item.Color,
			Silhouette: item.Silhouette,
			Material:   item.Material,
			Season:     item.Season,
			Notes:      item.Notes,
		})
	}
	if len(wardrobeInputs) == 0 {
		return parsedSubmitDraft{}, ValidationError{Field: "wardrobe.items", Message: "required"}
	}

	return parsedSubmitDraft{
		basic: basicMap,
		profile: profile.OnboardingInput{
			Gender:             basic.Gender,
			HeightCM:           basic.HeightCM,
			BodyNotes:          basic.BodyNotes,
			SkinNotes:          basic.SkinNotes,
			HairNotes:          basic.HairNotes,
			LifestyleScenarios: scenarios,
			StyleGoals:         cleanStrings(style.Goals),
			Avoidances:         cleanStrings(style.Avoidances),
		},
		assets:            parseAssetInputs(data),
		wardrobe:          wardrobeInputs,
		generatorWardrobe: generatorInputs,
		referenceStyles:   cleanStrings(style.ReferenceStyles),
	}, nil
}

func decodeRequired(data DraftData, key string, target any) error {
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ValidationError{Field: key, Message: "required"}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}

type basicDraft struct {
	Gender    string `json:"gender"`
	HeightCM  *int   `json:"height_cm"`
	BodyNotes string `json:"body_notes"`
	SkinNotes string `json:"skin_notes"`
	HairNotes string `json:"hair_notes"`
}

type styleGoalDraft struct {
	Goals           []string `json:"goals"`
	Avoidances      []string `json:"avoidances"`
	Scenarios       []string `json:"scenarios"`
	ReferenceStyles []string `json:"reference_styles"`
}

type wardrobeDraft struct {
	Items []wardrobeItemDraft `json:"items"`
}

type wardrobeItemDraft struct {
	Name       string `json:"name"`
	Category   string `json:"category"`
	Color      string `json:"color"`
	Silhouette string `json:"silhouette"`
	Material   string `json:"material"`
	Season     string `json:"season"`
	Notes      string `json:"notes"`
}

type assetDraft struct {
	ObjectKey string `json:"object_key"`
	MimeType  string `json:"mime_type"`
	FileSize  int64  `json:"file_size"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
	AssetType string `json:"asset_type"`
}

func parseAssetInputs(data DraftData) []asset.Input {
	keys := []string{"photos", "assets", "images"}
	var inputs []asset.Input
	for _, key := range keys {
		raw, ok := data[key]
		if !ok {
			continue
		}
		var wrapper struct {
			Items []assetDraft `json:"items"`
		}
		if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Items) > 0 {
			inputs = append(inputs, assetDraftsToInputs(wrapper.Items)...)
			continue
		}
		var list []assetDraft
		if err := json.Unmarshal(raw, &list); err == nil {
			inputs = append(inputs, assetDraftsToInputs(list)...)
		}
	}
	return inputs
}

func assetDraftsToInputs(items []assetDraft) []asset.Input {
	inputs := make([]asset.Input, 0, len(items))
	for _, item := range items {
		inputs = append(inputs, asset.Input{
			ObjectKey: item.ObjectKey,
			MimeType:  item.MimeType,
			FileSize:  item.FileSize,
			Width:     item.Width,
			Height:    item.Height,
			AssetType: item.AssetType,
		})
	}
	return inputs
}

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}
