package asset

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hestia/server/internal/common/id"
	"hestia/server/internal/infra/config"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/client"
	"github.com/qiniu/go-sdk/v7/storage"
)

const (
	DefaultBucket       = "local-onboarding"
	SourceOnboarding    = "onboarding"
	SourceMiniappUpload = "miniapp_upload"
	StatusActive        = "active"
	ReviewStatusPending = "pending"

	defaultQiniuUploadHost = "https://upload.qiniup.com"
	maxImageFileSize       = 10 * 1024 * 1024
	maxImageDimension      = 20000
	qiniuStatTimeout       = 5 * time.Second
	qiniuPreviewQuery      = "imageView2/2/w/360/h/360/q/80/format/webp"
)

var (
	ErrUnsupportedAssetType = errors.New("unsupported asset type")
	ErrInvalidMimeType      = errors.New("invalid mime type")
	ErrInvalidFileSize      = errors.New("invalid file size")
	ErrInvalidAssetRequest  = errors.New("invalid asset request")
	ErrAssetStorageNotReady = errors.New("asset storage is not configured")
	ErrAssetOwnership       = errors.New("asset ownership mismatch")
	ErrAssetNotFound        = errors.New("asset not found")
	ErrAssetAlreadyExists   = errors.New("asset already exists")
)

type Repository interface {
	CreateMany(ctx context.Context, items []Asset) ([]Asset, error)
}

type uploadRepository interface {
	Repository
	Create(ctx context.Context, item Asset) (Asset, error)
	FindByPublicID(ctx context.Context, publicID string) (Asset, error)
}

type ServiceOptions struct {
	Bucket            string
	UploadHost        string
	PrivateDomain     string
	UploadTTL         time.Duration
	DownloadTTL       time.Duration
	UploadSigner      UploadSigner
	DownloadSigner    DownloadSigner
	ObjectStatChecker ObjectStatChecker
}

type UploadSignRequest struct {
	Bucket    string
	ObjectKey string
	MimeType  string
	FileSize  int64
	TTL       time.Duration
}

type UploadSigner interface {
	SignUpload(ctx context.Context, req UploadSignRequest) (string, error)
}

type UploadSignerFunc func(ctx context.Context, req UploadSignRequest) (string, error)

func (f UploadSignerFunc) SignUpload(ctx context.Context, req UploadSignRequest) (string, error) {
	return f(ctx, req)
}

type DownloadSignRequest struct {
	PrivateDomain string
	ObjectKey     string
	Query         string
	TTL           time.Duration
}

type SignedImageURLs = struct {
	PreviewURL  string
	OriginalURL string
}

type DownloadSigner interface {
	SignDownload(ctx context.Context, req DownloadSignRequest) (string, error)
}

type DownloadSignerFunc func(ctx context.Context, req DownloadSignRequest) (string, error)

func (f DownloadSignerFunc) SignDownload(ctx context.Context, req DownloadSignRequest) (string, error) {
	return f(ctx, req)
}

type ObjectStat struct {
	FileSize int64
	MimeType string
}

type ObjectStatChecker interface {
	StatObject(ctx context.Context, bucket string, objectKey string) (ObjectStat, error)
}

type ObjectStatCheckerFunc func(ctx context.Context, bucket string, objectKey string) (ObjectStat, error)

func (f ObjectStatCheckerFunc) StatObject(ctx context.Context, bucket string, objectKey string) (ObjectStat, error) {
	return f(ctx, bucket, objectKey)
}

type Service struct {
	repo    Repository
	options ServiceOptions
}

func NewService(repo Repository) *Service {
	return NewServiceWithOptions(repo, ServiceOptions{})
}

func NewServiceWithOptions(repo Repository, options ServiceOptions) *Service {
	options.Bucket = strings.TrimSpace(options.Bucket)
	options.UploadHost = normalizeHTTPSURL(options.UploadHost)
	if options.UploadHost == "" {
		options.UploadHost = defaultQiniuUploadHost
	}
	options.PrivateDomain = normalizeHTTPSURL(options.PrivateDomain)
	if options.UploadTTL <= 0 {
		options.UploadTTL = time.Hour
	}
	if options.DownloadTTL <= 0 {
		options.DownloadTTL = 15 * time.Minute
	}
	return &Service{repo: repo, options: options}
}

func (s *Service) Options() ServiceOptions {
	if s == nil {
		return ServiceOptions{}
	}
	return s.options
}

func NewServiceFromConfig(repo Repository, cfg *config.Config) *Service {
	if cfg == nil {
		return NewServiceWithOptions(repo, ServiceOptions{})
	}
	options := ServiceOptions{
		Bucket:        cfg.QiniuBucket,
		UploadHost:    cfg.QiniuUploadHost,
		PrivateDomain: cfg.QiniuPrivateDomain,
		UploadTTL:     time.Duration(cfg.QiniuUploadTokenTTLSeconds) * time.Second,
		DownloadTTL:   time.Duration(cfg.QiniuDownloadURLTTLSeconds) * time.Second,
	}
	if strings.TrimSpace(cfg.QiniuAccessKey) != "" && strings.TrimSpace(cfg.QiniuSecretKey) != "" {
		credentials := qbox.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)
		qiniuClient := &client.Client{
			Client: &http.Client{
				Timeout:   qiniuStatTimeout,
				Transport: client.DefaultTransport,
			},
		}
		options.UploadSigner = qiniuUploadSigner{credentials: credentials}
		options.DownloadSigner = qiniuDownloadSigner{credentials: credentials}
		options.ObjectStatChecker = qiniuObjectStatChecker{
			manager: storage.NewBucketManagerEx(credentials, nil, qiniuClient),
		}
	}
	return NewServiceWithOptions(repo, options)
}

func (s *Service) RegisterOnboardingAssets(ctx context.Context, userID int64, inputs []Input) ([]Asset, error) {
	if len(inputs) == 0 {
		return []Asset{}, nil
	}
	items := make([]Asset, 0, len(inputs))
	for _, input := range inputs {
		objectKey := strings.TrimSpace(input.ObjectKey)
		assetPublicID := strings.TrimSpace(input.AssetPublicID)
		clientRef := strings.TrimSpace(input.ClientRef)
		if objectKey == "" && assetPublicID == "" && clientRef == "" {
			continue
		}
		if objectKey == "" {
			objectKey = simulatedObjectKey(assetPublicID, clientRef)
		}
		publicID := assetPublicID
		if publicID == "" {
			publicID = id.NewPublicID("ast")
		}
		mimeType := strings.TrimSpace(input.MimeType)
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		assetType := strings.TrimSpace(input.AssetType)
		if assetType == "" {
			assetType = "onboarding_photo"
		}
		items = append(items, Asset{
			PublicID:     publicID,
			ClientRef:    clientRef,
			OwnerUserID:  userID,
			Bucket:       DefaultBucket,
			ObjectKey:    objectKey,
			MimeType:     mimeType,
			FileSize:     input.FileSize,
			Width:        input.Width,
			Height:       input.Height,
			AssetType:    assetType,
			Source:       SourceOnboarding,
			Status:       StatusActive,
			ReviewStatus: ReviewStatusPending,
			Note:         strings.TrimSpace(input.Note),
			Metadata: map[string]any{
				"client_ref":                clientRef,
				"note":                      strings.TrimSpace(input.Note),
				"original_asset_public_id":  assetPublicID,
				"simulated_local_asset":     input.ObjectKey == "",
				"onboarding_source_version": 1,
			},
		})
	}
	if len(items) == 0 {
		return []Asset{}, nil
	}
	return s.repo.CreateMany(ctx, items)
}

func (s *Service) CreateUploadToken(ctx context.Context, userID int64, input UploadTokenInput) (UploadTokenResult, error) {
	if s == nil || s.options.Bucket == "" || s.options.UploadHost == "" || s.options.UploadSigner == nil {
		return UploadTokenResult{}, ErrAssetStorageNotReady
	}
	_, scope, err := normalizeAssetType(input.AssetType)
	if err != nil {
		return UploadTokenResult{}, err
	}
	mimeType, ext, err := normalizeImageMimeType(input.MimeType)
	if err != nil {
		return UploadTokenResult{}, err
	}
	if err := validateFileSize(input.FileSize); err != nil {
		return UploadTokenResult{}, err
	}
	publicID := id.NewPublicID("ast")
	objectKey := fmt.Sprintf("users/%d/%s/%s.%s", userID, scope, publicID, ext)
	token, err := s.options.UploadSigner.SignUpload(ctx, UploadSignRequest{
		Bucket:    s.options.Bucket,
		ObjectKey: objectKey,
		MimeType:  mimeType,
		FileSize:  input.FileSize,
		TTL:       s.options.UploadTTL,
	})
	if err != nil {
		return UploadTokenResult{}, err
	}
	expiresAt := time.Now().Add(s.options.UploadTTL).UTC().Format(time.RFC3339)
	return UploadTokenResult{
		AssetPublicID: publicID,
		FilePublicID:  publicID,
		Bucket:        s.options.Bucket,
		ObjectKey:     objectKey,
		UploadURL:     s.options.UploadHost,
		UploadToken:   token,
		ExpiresAt:     expiresAt,
	}, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, userID int64, input ConfirmInput) (ConfirmResult, error) {
	if s == nil {
		return ConfirmResult{}, ErrAssetStorageNotReady
	}
	repo, ok := s.repo.(uploadRepository)
	if !ok {
		return ConfirmResult{}, ErrAssetStorageNotReady
	}
	if s.options.Bucket == "" || s.options.PrivateDomain == "" || s.options.DownloadSigner == nil {
		return ConfirmResult{}, ErrAssetStorageNotReady
	}
	assetType, scope, err := normalizeAssetType(input.AssetType)
	if err != nil {
		return ConfirmResult{}, err
	}
	mimeType, ext, err := normalizeImageMimeType(input.MimeType)
	if err != nil {
		return ConfirmResult{}, err
	}
	if err := validateFileSize(input.FileSize); err != nil {
		return ConfirmResult{}, err
	}
	publicID := strings.TrimSpace(input.AssetPublicID)
	bucket := strings.TrimSpace(input.Bucket)
	objectKey := strings.TrimSpace(input.ObjectKey)
	if !validAssetPublicID(publicID) || bucket == "" || objectKey == "" {
		return ConfirmResult{}, ErrInvalidAssetRequest
	}
	if !validDimensions(input.Width, input.Height) {
		return ConfirmResult{}, ErrInvalidAssetRequest
	}
	if s.options.Bucket != "" && bucket != s.options.Bucket {
		return ConfirmResult{}, ErrAssetOwnership
	}
	expectedObjectKey := fmt.Sprintf("users/%d/%s/%s.%s", userID, scope, publicID, ext)
	if objectKey != expectedObjectKey {
		return ConfirmResult{}, ErrAssetOwnership
	}

	existing, err := repo.FindByPublicID(ctx, publicID)
	if err == nil {
		if existing.OwnerUserID != userID || existing.Bucket != bucket || existing.ObjectKey != objectKey {
			return ConfirmResult{}, ErrAssetOwnership
		}
		return s.confirmResult(ctx, existing)
	}
	if !errors.Is(err, ErrAssetNotFound) {
		return ConfirmResult{}, err
	}
	if err := s.verifyObject(ctx, bucket, objectKey, mimeType, input.FileSize); err != nil {
		return ConfirmResult{}, err
	}

	created := Asset{
		PublicID:     publicID,
		OwnerUserID:  userID,
		Bucket:       bucket,
		ObjectKey:    objectKey,
		MimeType:     mimeType,
		FileSize:     input.FileSize,
		Width:        input.Width,
		Height:       input.Height,
		AssetType:    assetType,
		Source:       SourceMiniappUpload,
		Status:       StatusActive,
		ReviewStatus: ReviewStatusPending,
		Metadata: map[string]any{
			"upload_source": SourceMiniappUpload,
			"confirmed_at":  time.Now().UTC().Format(time.RFC3339),
		},
	}
	if input.Width != nil {
		created.Metadata["width"] = *input.Width
	}
	if input.Height != nil {
		created.Metadata["height"] = *input.Height
	}
	created, err = repo.Create(ctx, created)
	if errors.Is(err, ErrAssetAlreadyExists) {
		existing, findErr := repo.FindByPublicID(ctx, publicID)
		if findErr != nil {
			return ConfirmResult{}, findErr
		}
		if existing.OwnerUserID != userID || existing.Bucket != bucket || existing.ObjectKey != objectKey {
			return ConfirmResult{}, ErrAssetOwnership
		}
		return s.confirmResult(ctx, existing)
	}
	if err != nil {
		return ConfirmResult{}, err
	}
	return s.confirmResult(ctx, created)
}

func (s *Service) PrivateDownloadURL(ctx context.Context, objectKey string) (string, error) {
	return s.privateDownloadURL(ctx, objectKey, "")
}

func (s *Service) PrivateImageURLs(ctx context.Context, objectKeys []string) (map[string]SignedImageURLs, error) {
	result := make(map[string]SignedImageURLs, len(objectKeys))
	for _, objectKey := range objectKeys {
		objectKey = strings.TrimSpace(objectKey)
		if objectKey == "" {
			continue
		}
		originalURL, err := s.privateDownloadURL(ctx, objectKey, "")
		if err != nil {
			return nil, err
		}
		previewURL, err := s.privateDownloadURL(ctx, objectKey, qiniuPreviewQuery)
		if err != nil {
			return nil, err
		}
		result[objectKey] = SignedImageURLs{
			PreviewURL:  previewURL,
			OriginalURL: originalURL,
		}
	}
	return result, nil
}

func (s *Service) privateDownloadURL(ctx context.Context, objectKey string, query string) (string, error) {
	if s == nil || s.options.PrivateDomain == "" || s.options.DownloadSigner == nil {
		return "", ErrAssetStorageNotReady
	}
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return "", ErrAssetNotFound
	}
	return s.options.DownloadSigner.SignDownload(ctx, DownloadSignRequest{
		PrivateDomain: s.options.PrivateDomain,
		ObjectKey:     objectKey,
		Query:         strings.TrimSpace(query),
		TTL:           s.options.DownloadTTL,
	})
}

type qiniuUploadSigner struct {
	credentials *qbox.Mac
}

func (s qiniuUploadSigner) SignUpload(_ context.Context, req UploadSignRequest) (string, error) {
	if s.credentials == nil {
		return "", ErrAssetStorageNotReady
	}
	if req.FileSize <= 0 || req.FileSize > maxImageFileSize {
		return "", ErrInvalidFileSize
	}
	policy := storage.PutPolicy{
		Scope:      req.Bucket + ":" + req.ObjectKey,
		Expires:    uint64(req.TTL / time.Second),
		InsertOnly: 1,
		FsizeMin:   1,
		FsizeLimit: req.FileSize,
		MimeLimit:  "image/jpeg;image/png;image/webp;image/heic",
	}
	return policy.UploadToken(s.credentials), nil
}

type qiniuDownloadSigner struct {
	credentials *qbox.Mac
}

func (s qiniuDownloadSigner) SignDownload(_ context.Context, req DownloadSignRequest) (string, error) {
	if s.credentials == nil {
		return "", ErrAssetStorageNotReady
	}
	deadline := time.Now().Add(req.TTL).Unix()
	if strings.TrimSpace(req.Query) != "" {
		return storage.MakePrivateURLv2WithQueryString(s.credentials, req.PrivateDomain, req.ObjectKey, req.Query, deadline), nil
	}
	return storage.MakePrivateURLv2(s.credentials, req.PrivateDomain, req.ObjectKey, deadline), nil
}

type qiniuObjectStatChecker struct {
	manager *storage.BucketManager
}

func (c qiniuObjectStatChecker) StatObject(ctx context.Context, bucket string, objectKey string) (ObjectStat, error) {
	if c.manager == nil {
		return ObjectStat{}, ErrAssetStorageNotReady
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ObjectStat{}, ctx.Err()
		default:
		}
	}
	info, err := c.manager.Stat(bucket, objectKey)
	if err != nil {
		var qiniuErr *client.ErrorInfo
		if errors.As(err, &qiniuErr) && (qiniuErr.HttpCode() == 404 || qiniuErr.Code == 612) {
			return ObjectStat{}, ErrAssetNotFound
		}
		return ObjectStat{}, err
	}
	return ObjectStat{FileSize: info.Fsize, MimeType: info.MimeType}, nil
}

func normalizeAssetType(value string) (assetType string, scope string, err error) {
	switch strings.TrimSpace(value) {
	case "clothes_item_photo":
		return "clothes_item_photo", "clothes", nil
	case "hair_photo":
		return "hair_photo", "hair", nil
	case "makeup_photo":
		return "makeup_photo", "makeup", nil
	case "onboarding_photo":
		return "onboarding_photo", "onboarding", nil
	case "style_reference":
		return "style_reference", "style-reference", nil
	case "chat_image":
		return "chat_image", "chat", nil
	default:
		return "", "", ErrUnsupportedAssetType
	}
}

func (s *Service) verifyObject(ctx context.Context, bucket string, objectKey string, mimeType string, fileSize int64) error {
	if s.options.ObjectStatChecker == nil {
		return nil
	}
	stat, err := s.options.ObjectStatChecker.StatObject(ctx, bucket, objectKey)
	if err != nil {
		return err
	}
	if stat.FileSize != fileSize {
		return ErrInvalidFileSize
	}
	if strings.TrimSpace(stat.MimeType) != "" && strings.ToLower(strings.TrimSpace(stat.MimeType)) != mimeType {
		return ErrInvalidMimeType
	}
	return nil
}

func (s *Service) confirmResult(ctx context.Context, item Asset) (ConfirmResult, error) {
	result := ConfirmResult{
		AssetPublicID: item.PublicID,
		FilePublicID:  item.PublicID,
		ObjectKey:     item.ObjectKey,
		AssetType:     item.AssetType,
		FileType:      item.AssetType,
	}
	return result, nil
}

func validAssetPublicID(publicID string) bool {
	if len(publicID) != 30 || !strings.HasPrefix(publicID, "ast_") {
		return false
	}
	for _, ch := range publicID[4:] {
		if (ch < 'a' || ch > 'z') && (ch < '2' || ch > '7') {
			return false
		}
	}
	return true
}

func validDimensions(width *int, height *int) bool {
	if width != nil && (*width <= 0 || *width > maxImageDimension) {
		return false
	}
	if height != nil && (*height <= 0 || *height > maxImageDimension) {
		return false
	}
	return true
}

func normalizeHTTPSURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") {
		return value
	}
	if strings.HasPrefix(lower, "http://") {
		return "https://" + strings.TrimSpace(value[len("http://"):])
	}
	return "https://" + value
}

func normalizeImageMimeType(value string) (mimeType string, ext string, err error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg":
		return "image/jpeg", "jpg", nil
	case "image/png":
		return "image/png", "png", nil
	case "image/webp":
		return "image/webp", "webp", nil
	case "image/heic":
		return "image/heic", "heic", nil
	default:
		return "", "", ErrInvalidMimeType
	}
}

func validateFileSize(value int64) error {
	if value <= 0 || value > maxImageFileSize {
		return ErrInvalidFileSize
	}
	return nil
}

func simulatedObjectKey(assetPublicID string, clientRef string) string {
	if assetPublicID != "" {
		return "simulated/asset-public-id/" + safeObjectKeyPart(assetPublicID)
	}
	return "simulated/client-ref/" + safeObjectKeyPart(clientRef)
}

func safeObjectKeyPart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}
