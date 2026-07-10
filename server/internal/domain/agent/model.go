package agent

import "time"

const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"

	ChatMsgTypeText      = "text"
	ChatMsgTypeDraftCard = "draft_card"
	ChatMsgTypeError     = "error"

	ChatStatusSent       = "sent"
	ChatStatusGenerating = "generating"
	ChatStatusFailed     = "failed"

	AgentStepTypeModelDecision = "model_decision"
	AgentStepTypeToolCall      = "tool_call"
	AgentStepTypeToolResult    = "tool_result"
	AgentStepTypeFinalResponse = "final_response"
	AgentStepStatusSucceeded   = "succeeded"
	AgentStepStatusFailed      = "failed"

	DraftStatusDraft     = "draft"
	DraftStatusConfirmed = "confirmed"
	DraftStatusDiscarded = "discarded"

	AdviceStatusReady = "ready"

	SectionTypeOutfit = "outfit"
	SectionTypeHair   = "hair"
	SectionTypeMakeup = "makeup"
)

type ChatRequest struct {
	Text string `json:"text"`
}

type StreamDone struct {
	JobPublicID     string `json:"job_public_id"`
	MessagePublicID string `json:"message_public_id"`
	DraftPublicID   string `json:"draft_public_id,omitempty"`
}

type StreamStatus struct {
	Text string `json:"text"`
}

type StreamMessage struct {
	Text string `json:"text"`
}

type ChatResult struct {
	Message                  StreamMessage
	Draft                    *DraftCard
	UserMessageID            int64
	UserMessagePublicID      string
	AssistantMessageID       int64
	AssistantMessagePublicID string
}

type AdviceRunInput struct {
	UserID         int64
	Text           string
	SourceMsgID    int64
	RecentMessages []ChatMessage
	CurrentDraft   *Draft
}

type AdviceRunOutput struct {
	AssistantText string
	DecisionLabel string
	Metadata      AdviceRunMetadata
	AuditSteps    []AdviceRunAuditStep
	ToolCalls     []AdviceToolCall
}

type AdviceRunMetadata struct {
	UsageKey      string
	ProviderCode  string
	ModelCode     string
	PromptVersion string
	MaxIterations int
}

type AdviceToolCall struct {
	Name             string
	ToolCallID       string
	InputSummary     string
	CreateDraftInput *CreateDraftInput
	UpdateDraftInput *UpdateDraftInput
}

type AdviceRunAuditStep struct {
	StepType        string
	Status          string
	ToolName        string
	ToolCallID      string
	DecisionLabel   string
	InputSummary    string
	OutputSummary   string
	RelatedType     string
	RelatedID       int64
	RelatedPublicID string
	ErrorMessage    string
	DurationMS      int
}

type ChatMessage struct {
	ID              int64
	PublicID        string
	UserID          int64
	SourceMsgID     int64
	Role            string
	MsgType         string
	ContentText     string
	RelatedType     string
	RelatedID       int64
	RelatedPublicID string
	Status          string
}

type CreateChatMessageInput struct {
	UserID          int64
	SourceMsgID     int64
	Role            string
	MsgType         string
	ContentText     string
	RelatedType     string
	RelatedID       int64
	RelatedPublicID string
	Status          string
}

type UpdateChatMessageInput struct {
	ID              int64
	Status          string
	MsgType         string
	ContentText     string
	RelatedType     string
	RelatedID       int64
	RelatedPublicID string
}

type AgentRunStepInput struct {
	UserID          int64
	SourceMsgID     int64
	AssistantMsgID  int64
	StepNo          int
	StepType        string
	Status          string
	UsageKey        string
	ProviderCode    string
	ModelCode       string
	PromptVersion   string
	MaxIterations   int
	ToolName        string
	ToolCallID      string
	DecisionLabel   string
	InputSummary    string
	OutputSummary   string
	RelatedType     string
	RelatedID       int64
	RelatedPublicID string
	StartedAt       time.Time
	FinishedAt      time.Time
	DurationMS      int
	ErrorMessage    string
}

type Draft struct {
	ID                int64          `json:"-"`
	PublicID          string         `json:"public_id"`
	UserID            int64          `json:"-"`
	SourceMsgID       int64          `json:"-"`
	Status            string         `json:"status"`
	SceneKey          string         `json:"scene_key,omitempty"`
	SceneLabel        string         `json:"scene_label,omitempty"`
	TargetDate        *time.Time     `json:"target_date,omitempty"`
	Occasion          string         `json:"occasion,omitempty"`
	WeatherText       string         `json:"weather_text,omitempty"`
	MoodText          string         `json:"mood_text,omitempty"`
	StyleGoal         string         `json:"style_goal,omitempty"`
	AvoidGoal         string         `json:"avoid_goal,omitempty"`
	CurrentRevisionNo int            `json:"current_revision_no"`
	ConfirmedAdviceID int64          `json:"-"`
	Sections          []DraftSection `json:"sections,omitempty"`
	CreatedAt         time.Time      `json:"created_at,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at,omitempty"`
}

type DraftSection struct {
	ID                      int64          `json:"-"`
	PublicID                string         `json:"public_id"`
	DraftID                 int64          `json:"-"`
	UserID                  int64          `json:"-"`
	SectionType             string         `json:"section_type"`
	CurrentSectionVersionID int64          `json:"-"`
	CurrentSectionVersionNo int            `json:"current_section_version_no"`
	ContentSchemaVersion    string         `json:"content_schema_version"`
	ContentJSON             map[string]any `json:"content_json"`
	CreatedAt               time.Time      `json:"created_at,omitempty"`
	UpdatedAt               time.Time      `json:"updated_at,omitempty"`
}

type DraftSectionVersion struct {
	ID                   int64          `json:"-"`
	PublicID             string         `json:"public_id"`
	DraftID              int64          `json:"-"`
	SectionID            int64          `json:"-"`
	UserID               int64          `json:"-"`
	SectionType          string         `json:"section_type"`
	SectionVersionNo     int            `json:"section_version_no"`
	DraftRevisionNo      int            `json:"draft_revision_no"`
	SourceMsgID          int64          `json:"-"`
	UserIntent           string         `json:"user_intent,omitempty"`
	RevisionSummary      string         `json:"revision_summary,omitempty"`
	ContentSchemaVersion string         `json:"content_schema_version"`
	ContentJSON          map[string]any `json:"content_json"`
	CreatedAt            time.Time      `json:"created_at,omitempty"`
}

type Advice struct {
	ID                    int64           `json:"-"`
	PublicID              string          `json:"public_id"`
	UserID                int64           `json:"-"`
	SourceMsgID           int64           `json:"-"`
	SourceDraftID         int64           `json:"-"`
	SourceDraftRevisionNo int             `json:"source_draft_revision_no"`
	Status                string          `json:"status"`
	SceneKey              string          `json:"scene_key,omitempty"`
	SceneLabel            string          `json:"scene_label,omitempty"`
	TargetDate            *time.Time      `json:"target_date,omitempty"`
	Occasion              string          `json:"occasion,omitempty"`
	WeatherText           string          `json:"weather_text,omitempty"`
	MoodText              string          `json:"mood_text,omitempty"`
	StyleGoal             string          `json:"style_goal,omitempty"`
	AvoidGoal             string          `json:"avoid_goal,omitempty"`
	Sections              []AdviceSection `json:"sections,omitempty"`
	CreatedAt             time.Time       `json:"created_at,omitempty"`
	UpdatedAt             time.Time       `json:"updated_at,omitempty"`
}

type AdviceSection struct {
	ID                          int64          `json:"-"`
	PublicID                    string         `json:"public_id"`
	AdviceID                    int64          `json:"-"`
	UserID                      int64          `json:"-"`
	SectionType                 string         `json:"section_type"`
	SourceDraftSectionVersionID int64          `json:"-"`
	ContentSchemaVersion        string         `json:"content_schema_version"`
	ContentJSON                 map[string]any `json:"content_json"`
	CreatedAt                   time.Time      `json:"created_at,omitempty"`
	UpdatedAt                   time.Time      `json:"updated_at,omitempty"`
}

type CreateDraftInput struct {
	UserID          int64
	SourceMsgID     int64
	TargetDate      *time.Time
	SceneKey        string
	SceneLabel      string
	Occasion        string
	WeatherText     string
	MoodText        string
	StyleGoal       string
	AvoidGoal       string
	Sections        []DraftSectionInput
	UserIntent      string
	RevisionSummary string
}

type UpdateDraftInput struct {
	UserID          int64
	PublicID        string
	SourceMsgID     int64
	Sections        []DraftSectionInput
	UserIntent      string
	RevisionSummary string
}

type DraftSectionInput struct {
	SectionType          string
	ContentSchemaVersion string
	ContentJSON          map[string]any
	RevisionSummary      string
}

type DraftCard struct {
	DraftPublicID string         `json:"draft_public_id"`
	RevisionNo    int            `json:"revision_no"`
	Status        string         `json:"status"`
	SceneLabel    string         `json:"scene_label,omitempty"`
	Sections      []DraftSection `json:"sections"`
}

type DraftRevision struct {
	DraftRevisionNo int                   `json:"draft_revision_no"`
	SourceMsgID     int64                 `json:"-"`
	UserIntent      string                `json:"user_intent,omitempty"`
	RevisionSummary string                `json:"revision_summary,omitempty"`
	Sections        []DraftVersionSection `json:"sections"`
	CreatedAt       time.Time             `json:"created_at,omitempty"`
}

type DraftVersionSection struct {
	PublicID             string         `json:"public_id"`
	SectionType          string         `json:"section_type"`
	SectionVersionNo     int            `json:"section_version_no"`
	ContentSchemaVersion string         `json:"content_schema_version"`
	ContentJSON          map[string]any `json:"content_json"`
	CreatedAt            time.Time      `json:"created_at,omitempty"`
}
