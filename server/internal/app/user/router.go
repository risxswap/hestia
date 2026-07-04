package user

import (
	"context"
	"time"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"
	"hestia/server/internal/domain/account"
	"hestia/server/internal/domain/agent"
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/onboarding"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"
	"hestia/server/internal/domain/wardrobe"
	"hestia/server/internal/infra/llm"

	"github.com/gin-gonic/gin"
)

func NewRouter(deps *baseapp.Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	api := router.Group("/api/user")
	api.GET("/health", health)
	account.RegisterUserRoutes(api, deps)
	protected := api.Group("")
	if deps == nil {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(nil)))
	} else {
		protected.Use(auth.RequireUserSession(auth.NewRedisSessionStore(deps.Redis)))
	}
	registerFileRoutes(protected.Group("/files"), deps)
	agent.RegisterUserRoutes(protected.Group("/agent"), deps)
	onboarding.RegisterUserRoutes(protected.Group("/onboarding"), deps)
	profile.RegisterUserRoutes(protected.Group("/profile"), deps)
	job.RegisterUserRoutes(protected.Group("/jobs"), deps)
	report.RegisterUserRoutes(protected.Group("/reports"), deps)
	imageroute.RegisterUserRoutes(protected.Group("/image-routes"), deps)
	wardrobe.RegisterUserRoutes(protected.Group("/wardrobe"), deps)
	return router
}

func registerFileRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	if deps == nil {
		asset.RegisterFileRoutes(group, deps)
		return
	}
	repo := asset.NewMySQLRepository(deps.DB)
	baseFileService := asset.NewServiceFromConfig(repo, deps.Config)
	options := baseFileService.Options()
	if deps.LLM != nil {
		wardrobeRepo := wardrobe.NewMySQLRepository(deps.DB)
		wardrobeService := wardrobe.NewService(wardrobeRepo)
		llmRepo := llm.NewMySQLConfigRepository(deps.DB)
		wardrobeService.SetImageRecognizer(wardrobe.NewLLMImageRecognizer(llm.NewService(llm.NewConfigResolver(llmRepo), deps.LLM)))
		options.UploadConfirmer = asset.UploadConfirmerFunc(func(ctx context.Context, userID int64, upload asset.ConfirmedUpload) (asset.UploadConfirmResult, error) {
			if upload.FileType != "wardrobe_item_photo" {
				return asset.UploadConfirmResult{}, nil
			}
			fields, err := wardrobeService.RecognizeItemImage(ctx, userID, wardrobe.RecognizeImageInput{
				AssetPublicID: upload.FilePublicID,
				ImageURL:      upload.URL,
			})
			if err != nil {
				return asset.UploadConfirmResult{}, err
			}
			return asset.UploadConfirmResult{RecognizedFields: recognizedFieldsMap(fields)}, nil
		})
	}
	service := asset.NewServiceWithOptions(repo, options)
	asset.RegisterFileRoutesWithService(group, service, deps.Logger)
}

func recognizedFieldsMap(fields wardrobe.RecognizedItemFields) map[string]any {
	result := map[string]any{}
	if fields.Name != "" {
		result["name"] = fields.Name
	}
	if fields.Category != "" {
		result["category"] = fields.Category
	}
	if fields.Color != "" {
		result["color"] = fields.Color
	}
	if fields.Silhouette != "" {
		result["silhouette"] = fields.Silhouette
	}
	if fields.Material != "" {
		result["material"] = fields.Material
	}
	if fields.Season != "" {
		result["season"] = fields.Season
	}
	if len(fields.SceneTags) > 0 {
		result["scene_tags"] = fields.SceneTags
	}
	if fields.UserNotes != "" {
		result["user_notes"] = fields.UserNotes
	}
	if fields.Confidence > 0 {
		result["confidence"] = fields.Confidence
	}
	return result
}

func timeDurationSeconds(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func health(c *gin.Context) {
	response.OK(c, gin.H{
		"status":  "ok",
		"surface": "user",
	})
}
