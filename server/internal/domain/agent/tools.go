package agent

import (
	"context"
	"errors"

	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/memory"
	"hestia/server/internal/domain/profile"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

const (
	adviceToolSessionUserID      = "hestia_user_id"
	adviceToolSessionSourceMsgID = "hestia_source_msg_id"
)

type adviceToolContextKey string

const (
	adviceToolContextUserID      adviceToolContextKey = "hestia_user_id"
	adviceToolContextSourceMsgID adviceToolContextKey = "hestia_source_msg_id"
)

var ErrAdviceToolSessionMissing = errors.New("advice tool session missing")

func NewAdviceTools(repo Repository, clothesServices ...ClothesAdviceService) ([]tool.BaseTool, error) {
	var deps AdviceToolDependencies
	if len(clothesServices) > 0 {
		deps.Clothes = clothesServices[0]
	}
	return NewAdviceToolsWithDependencies(repo, deps)
}

func NewAdviceToolsWithDependencies(repo Repository, deps AdviceToolDependencies) ([]tool.BaseTool, error) {
	if repo == nil {
		return nil, ErrRepositoryUnsupported
	}
	getCurrentDraftTool, err := toolutils.InferTool[adviceToolEmptyInput, adviceToolDraftOutput](
		"get_current_advice_draft",
		"读取当前用户最近一条形象建议草稿，返回场景、版本和穿搭/发型/妆容分段。",
		func(ctx context.Context, _ adviceToolEmptyInput) (adviceToolDraftOutput, error) {
			userID, _, err := adviceToolSession(ctx)
			if err != nil {
				return adviceToolDraftOutput{}, err
			}
			draft, err := repo.GetCurrentDraft(ctx, userID)
			if err != nil {
				return adviceToolDraftOutput{OK: false, ErrorMessage: err.Error()}, nil
			}
			card := draftCard(draft)
			return adviceToolDraftOutput{OK: true, Draft: &card}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	createDraftTool, err := toolutils.InferTool[adviceToolCreateDraftInput, adviceToolDraftOutput](
		AdviceToolCreateDraft,
		"创建当前用户的形象建议草稿，必须一次提供 outfit、hair、makeup 三个分段。",
		func(ctx context.Context, input adviceToolCreateDraftInput) (adviceToolDraftOutput, error) {
			userID, sourceMsgID, err := adviceToolSession(ctx)
			if err != nil {
				return adviceToolDraftOutput{}, err
			}
			draft, err := repo.CreateDraft(ctx, input.domainInput(userID, sourceMsgID))
			if err != nil {
				return adviceToolDraftOutput{OK: false, ErrorMessage: err.Error()}, nil
			}
			card := draftCard(draft)
			return adviceToolDraftOutput{OK: true, Draft: &card}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	updateDraftTool, err := toolutils.InferTool[adviceToolUpdateDraftInput, adviceToolDraftOutput](
		AdviceToolUpdateDraft,
		"精修当前用户已有形象建议草稿，只传入需要修改的 outfit、hair 或 makeup 分段。",
		func(ctx context.Context, input adviceToolUpdateDraftInput) (adviceToolDraftOutput, error) {
			userID, sourceMsgID, err := adviceToolSession(ctx)
			if err != nil {
				return adviceToolDraftOutput{}, err
			}
			draft, err := repo.UpdateDraftSections(ctx, input.domainInput(userID, sourceMsgID))
			if err != nil {
				return adviceToolDraftOutput{OK: false, ErrorMessage: err.Error()}, nil
			}
			card := draftCard(draft)
			return adviceToolDraftOutput{OK: true, Draft: &card}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	discardDraftTool, err := toolutils.InferTool[adviceToolDiscardDraftInput, adviceToolDiscardOutput](
		"discard_advice_draft",
		"废弃当前用户指定的形象建议草稿。保存正式建议不能使用此工具。",
		func(ctx context.Context, input adviceToolDiscardDraftInput) (adviceToolDiscardOutput, error) {
			userID, _, err := adviceToolSession(ctx)
			if err != nil {
				return adviceToolDiscardOutput{}, err
			}
			if err := repo.DiscardDraft(ctx, userID, input.PublicID); err != nil {
				return adviceToolDiscardOutput{OK: false, ErrorMessage: err.Error()}, nil
			}
			return adviceToolDiscardOutput{OK: true, DraftPublicID: input.PublicID}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	tools := []tool.BaseTool{getCurrentDraftTool, createDraftTool, updateDraftTool, discardDraftTool}
	if deps.Profile != nil {
		profileTool, err := toolutils.InferTool[adviceToolEmptyInput, adviceToolProfileOutput](
			"get_profile_context",
			"读取当前用户的结构化形象档案、偏好、禁忌、记忆统计和最近报告摘要，供生成建议前参考。",
			func(ctx context.Context, _ adviceToolEmptyInput) (adviceToolProfileOutput, error) {
				userID, _, err := adviceToolSession(ctx)
				if err != nil {
					return adviceToolProfileOutput{}, err
				}
				summary, err := deps.Profile.Summary(ctx, userID)
				if err != nil {
					return adviceToolProfileOutput{OK: false, ErrorMessage: err.Error()}, nil
				}
				return adviceToolProfileContext(summary), nil
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, profileTool)
	}
	if deps.Memory != nil {
		memoryTool, err := toolutils.InferTool[adviceToolMemoryInput, adviceToolMemoryOutput](
			"get_memory_context",
			"按用户当前需求读取长期记忆片段，包括明确事实、偏好、禁忌和 AI 推断。",
			func(ctx context.Context, input adviceToolMemoryInput) (adviceToolMemoryOutput, error) {
				userID, _, err := adviceToolSession(ctx)
				if err != nil {
					return adviceToolMemoryOutput{}, err
				}
				limit := input.Limit
				if limit <= 0 || limit > 12 {
					limit = 8
				}
				items, err := deps.Memory.AgentMemoryContext(ctx, userID, input.Query, limit)
				if err != nil {
					return adviceToolMemoryOutput{OK: false, ErrorMessage: err.Error()}, nil
				}
				return adviceToolMemoryOutput{OK: true, Items: adviceToolMemoryItems(items)}, nil
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, memoryTool)
	}
	if deps.Clothes != nil {
		wardrobeTool, err := toolutils.InferTool[adviceToolWardrobeInput, adviceToolWardrobeOutput](
			"get_wardrobe_context",
			"按场景读取当前用户可用于建议的核心衣物，返回短字段供穿搭草稿引用。",
			func(ctx context.Context, input adviceToolWardrobeInput) (adviceToolWardrobeOutput, error) {
				userID, _, err := adviceToolSession(ctx)
				if err != nil {
					return adviceToolWardrobeOutput{}, err
				}
				limit := input.Limit
				if limit <= 0 || limit > 10 {
					limit = 5
				}
				items, err := deps.Clothes.AdviceContextItems(ctx, userID, clothes.AdviceContextFilter{
					Scene: input.Scene,
					Limit: limit,
				})
				if err != nil {
					return adviceToolWardrobeOutput{OK: false, ErrorMessage: err.Error()}, nil
				}
				return adviceToolWardrobeOutput{OK: true, Items: adviceToolWardrobeItems(items)}, nil
			},
		)
		if err != nil {
			return nil, err
		}
		tools = append(tools, wardrobeTool)
	}
	return tools, nil
}

func adviceToolSession(ctx context.Context) (int64, int64, error) {
	userID := sessionInt64(ctx, adviceToolSessionUserID, adviceToolContextUserID)
	sourceMsgID := sessionInt64(ctx, adviceToolSessionSourceMsgID, adviceToolContextSourceMsgID)
	if userID == 0 {
		return 0, 0, ErrAdviceToolSessionMissing
	}
	return userID, sourceMsgID, nil
}

func sessionInt64(ctx context.Context, sessionKey string, contextKey adviceToolContextKey) int64 {
	if value, ok := adk.GetSessionValue(ctx, sessionKey); ok {
		return anyInt64(value)
	}
	return anyInt64(ctx.Value(contextKey))
}

func anyInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

type adviceToolEmptyInput struct{}

type adviceToolDraftOutput struct {
	OK           bool       `json:"ok"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Draft        *DraftCard `json:"draft,omitempty"`
}

type adviceToolDiscardOutput struct {
	OK            bool   `json:"ok"`
	ErrorMessage  string `json:"error_message,omitempty"`
	DraftPublicID string `json:"draft_public_id,omitempty"`
}

type adviceToolWardrobeInput struct {
	Scene string `json:"scene,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type adviceToolWardrobeOutput struct {
	OK           bool                     `json:"ok"`
	ErrorMessage string                   `json:"error_message,omitempty"`
	Items        []adviceToolWardrobeItem `json:"items,omitempty"`
}

type adviceToolProfileOutput struct {
	OK            bool                         `json:"ok"`
	ErrorMessage  string                       `json:"error_message,omitempty"`
	User          profile.UserSummary          `json:"user"`
	Profile       *profile.ProfileSummary      `json:"profile,omitempty"`
	Preferences   profile.PreferencesSummary   `json:"preferences"`
	MemorySummary profile.MemorySummary        `json:"memory_summary"`
	LatestReport  *profile.LatestReportSummary `json:"latest_report,omitempty"`
	QuickEntries  []profile.QuickEntry         `json:"quick_entries,omitempty"`
}

type adviceToolMemoryInput struct {
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type adviceToolMemoryOutput struct {
	OK           bool                   `json:"ok"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Items        []adviceToolMemoryItem `json:"items,omitempty"`
}

type adviceToolMemoryItem struct {
	PublicID       string   `json:"public_id"`
	MemoryType     string   `json:"memory_type"`
	TypeLabel      string   `json:"type_label,omitempty"`
	MemoryKey      string   `json:"memory_key"`
	DisplayText    string   `json:"display_text"`
	Polarity       string   `json:"polarity"`
	Confidence     *float64 `json:"confidence,omitempty"`
	SourceLabel    string   `json:"source_label,omitempty"`
	CorrectionNote string   `json:"correction_note,omitempty"`
}

type adviceToolWardrobeItem struct {
	PublicID             string   `json:"public_id"`
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Color                string   `json:"color,omitempty"`
	Silhouette           string   `json:"silhouette,omitempty"`
	Material             string   `json:"material,omitempty"`
	Season               string   `json:"season,omitempty"`
	SceneTags            []string `json:"scene_tags,omitempty"`
	IsCore               bool     `json:"is_core"`
	RecommendationStatus string   `json:"recommendation_status"`
}

func adviceToolWardrobeItems(items []clothes.Item) []adviceToolWardrobeItem {
	result := make([]adviceToolWardrobeItem, 0, len(items))
	for _, item := range items {
		result = append(result, adviceToolWardrobeItem{
			PublicID:             item.PublicID,
			Name:                 item.Name,
			Category:             item.Category,
			Color:                item.Color,
			Silhouette:           item.Silhouette,
			Material:             item.Material,
			Season:               item.Season,
			SceneTags:            item.SceneTags,
			IsCore:               item.IsCore,
			RecommendationStatus: item.RecommendationStatus,
		})
	}
	return result
}

func adviceToolProfileContext(summary profile.Summary) adviceToolProfileOutput {
	return adviceToolProfileOutput{
		OK:            true,
		User:          summary.User,
		Profile:       summary.Profile,
		Preferences:   summary.Preferences,
		MemorySummary: summary.MemorySummary,
		LatestReport:  summary.LatestReport,
		QuickEntries:  summary.QuickEntries,
	}
}

func adviceToolMemoryItems(items []memory.Item) []adviceToolMemoryItem {
	result := make([]adviceToolMemoryItem, 0, len(items))
	for _, item := range items {
		decorated := memory.DecorateItem(item)
		result = append(result, adviceToolMemoryItem{
			PublicID:       decorated.PublicID,
			MemoryType:     decorated.MemoryType,
			TypeLabel:      decorated.TypeLabel,
			MemoryKey:      decorated.MemoryKey,
			DisplayText:    decorated.DisplayText,
			Polarity:       decorated.Polarity,
			Confidence:     decorated.Confidence,
			SourceLabel:    decorated.SourceLabel,
			CorrectionNote: decorated.CorrectionNote,
		})
	}
	return result
}

type adviceToolCreateDraftInput struct {
	SceneKey        string                   `json:"scene_key,omitempty"`
	SceneLabel      string                   `json:"scene_label,omitempty"`
	Occasion        string                   `json:"occasion,omitempty"`
	WeatherText     string                   `json:"weather_text,omitempty"`
	MoodText        string                   `json:"mood_text,omitempty"`
	StyleGoal       string                   `json:"style_goal,omitempty"`
	AvoidGoal       string                   `json:"avoid_goal,omitempty"`
	UserIntent      string                   `json:"user_intent,omitempty"`
	RevisionSummary string                   `json:"revision_summary,omitempty"`
	Sections        []adviceToolSectionInput `json:"sections"`
}

func (input adviceToolCreateDraftInput) domainInput(userID, sourceMsgID int64) CreateDraftInput {
	return CreateDraftInput{
		UserID:          userID,
		SourceMsgID:     sourceMsgID,
		SceneKey:        input.SceneKey,
		SceneLabel:      input.SceneLabel,
		Occasion:        input.Occasion,
		WeatherText:     input.WeatherText,
		MoodText:        input.MoodText,
		StyleGoal:       input.StyleGoal,
		AvoidGoal:       input.AvoidGoal,
		UserIntent:      input.UserIntent,
		RevisionSummary: input.RevisionSummary,
		Sections:        adviceToolSections(input.Sections).domainInputs(),
	}
}

type adviceToolUpdateDraftInput struct {
	PublicID        string                   `json:"public_id,omitempty"`
	UserIntent      string                   `json:"user_intent,omitempty"`
	RevisionSummary string                   `json:"revision_summary,omitempty"`
	Sections        []adviceToolSectionInput `json:"sections"`
}

func (input adviceToolUpdateDraftInput) domainInput(userID, sourceMsgID int64) UpdateDraftInput {
	return UpdateDraftInput{
		UserID:          userID,
		SourceMsgID:     sourceMsgID,
		PublicID:        input.PublicID,
		UserIntent:      input.UserIntent,
		RevisionSummary: input.RevisionSummary,
		Sections:        adviceToolSections(input.Sections).domainInputs(),
	}
}

type adviceToolDiscardDraftInput struct {
	PublicID string `json:"public_id"`
}

type adviceToolSectionInput struct {
	SectionType          string         `json:"section_type"`
	ContentSchemaVersion string         `json:"content_schema_version,omitempty"`
	ContentJSON          map[string]any `json:"content_json"`
	RevisionSummary      string         `json:"revision_summary,omitempty"`
}

type adviceToolSections []adviceToolSectionInput

func (sections adviceToolSections) domainInputs() []DraftSectionInput {
	result := make([]DraftSectionInput, 0, len(sections))
	for _, section := range sections {
		result = append(result, DraftSectionInput{
			SectionType:          section.SectionType,
			ContentSchemaVersion: section.ContentSchemaVersion,
			ContentJSON:          section.ContentJSON,
			RevisionSummary:      section.RevisionSummary,
		})
	}
	return result
}

func contextWithAdviceToolSession(ctx context.Context, userID, sourceMsgID int64) context.Context {
	ctx = context.WithValue(ctx, adviceToolContextUserID, userID)
	ctx = context.WithValue(ctx, adviceToolContextSourceMsgID, sourceMsgID)
	return ctx
}
