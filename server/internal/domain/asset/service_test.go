package asset

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/qiniu/go-sdk/v7/auth/qbox"
)

type captureRepo struct {
	items []Asset
}

func (r *captureRepo) CreateMany(_ context.Context, items []Asset) ([]Asset, error) {
	r.items = append(r.items, items...)
	return items, nil
}

func TestRegisterOnboardingAssetsPersistsReferenceMetadata(t *testing.T) {
	repo := &captureRepo{}
	_, err := NewService(repo).RegisterOnboardingAssets(context.Background(), 12, []Input{
		{
			AssetPublicID: "ast_existing",
			ClientRef:     "tmp-1",
			AssetType:     "selfie",
			Note:          "自然光自拍",
		},
	})
	if err != nil {
		t.Fatalf("register assets: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected one asset, got %#v", repo.items)
	}

	metadata := repo.items[0].Metadata
	if metadata["client_ref"] != "tmp-1" {
		t.Fatalf("expected client_ref metadata, got %#v", metadata)
	}
	if metadata["note"] != "自然光自拍" {
		t.Fatalf("expected note metadata, got %#v", metadata)
	}
	if metadata["original_asset_public_id"] != "ast_existing" {
		t.Fatalf("expected original asset public id metadata, got %#v", metadata)
	}
}

func TestQiniuUploadSignerLimitsTokenToRequestedFileSize(t *testing.T) {
	token, err := qiniuUploadSigner{credentials: qboxTestCredentials()}.SignUpload(context.Background(), UploadSignRequest{
		Bucket:    "private-assets",
		ObjectKey: "users/12/clothes/ast_test.jpg",
		MimeType:  "image/jpeg",
		FileSize:  2048,
	})
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}

	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		t.Fatalf("expected qiniu upload token with three parts, got %q", token)
	}
	raw, err := decodeQiniuPolicy(parts[2])
	if err != nil {
		t.Fatalf("decode put policy: %v", err)
	}
	var policy map[string]any
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatalf("unmarshal put policy: %v", err)
	}
	if policy["fsizeLimit"] != float64(2048) {
		t.Fatalf("expected fsizeLimit 2048, got %#v in %s", policy["fsizeLimit"], string(raw))
	}
}

func TestQiniuUploadSignerRejectsInvalidFileSize(t *testing.T) {
	tests := []struct {
		name     string
		fileSize int64
	}{
		{name: "zero", fileSize: 0},
		{name: "too large", fileSize: maxImageFileSize + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := qiniuUploadSigner{credentials: qboxTestCredentials()}.SignUpload(context.Background(), UploadSignRequest{
				Bucket:    "private-assets",
				ObjectKey: "users/12/clothes/ast_test.jpg",
				MimeType:  "image/jpeg",
				FileSize:  tt.fileSize,
			})
			if err != ErrInvalidFileSize {
				t.Fatalf("expected ErrInvalidFileSize, got %v", err)
			}
		})
	}
}

func TestNewServiceWithOptionsNormalizesAssetURLsToHTTPS(t *testing.T) {
	service := NewServiceWithOptions(&captureRepo{}, ServiceOptions{
		Bucket:        "private-assets",
		UploadHost:    "http://upload.example.test",
		PrivateDomain: "private.example.test",
	})

	if service.options.UploadHost != "https://upload.example.test" {
		t.Fatalf("expected upload host to be normalized to https, got %q", service.options.UploadHost)
	}
	if service.options.PrivateDomain != "https://private.example.test" {
		t.Fatalf("expected private domain to be normalized to https, got %q", service.options.PrivateDomain)
	}
}

func TestNewServiceWithOptionsDefaultsQiniuUploadHost(t *testing.T) {
	service := NewServiceWithOptions(&captureRepo{}, ServiceOptions{})

	if service.options.UploadHost != "https://upload.qiniup.com" {
		t.Fatalf("expected default qiniu upload host, got %q", service.options.UploadHost)
	}
}

func TestPrivateImageURLsSignsOriginalAndPreviewURLs(t *testing.T) {
	var requests []DownloadSignRequest
	service := NewServiceWithOptions(&captureRepo{}, ServiceOptions{
		Bucket:        "private-assets",
		PrivateDomain: "private.example.test",
		DownloadSigner: DownloadSignerFunc(func(_ context.Context, req DownloadSignRequest) (string, error) {
			requests = append(requests, req)
			if req.Query != "" {
				return req.PrivateDomain + "/" + req.ObjectKey + "?" + req.Query + "&token=preview", nil
			}
			return req.PrivateDomain + "/" + req.ObjectKey + "?token=original", nil
		}),
	})

	urls, err := service.PrivateImageURLs(context.Background(), []string{"users/12/clothes/ast_test.jpg"})
	if err != nil {
		t.Fatalf("private image urls: %v", err)
	}

	got := urls["users/12/clothes/ast_test.jpg"]
	if got.OriginalURL != "https://private.example.test/users/12/clothes/ast_test.jpg?token=original" {
		t.Fatalf("expected original url, got %#v", got)
	}
	if got.PreviewURL != "https://private.example.test/users/12/clothes/ast_test.jpg?imageView2/2/w/360/h/360/q/80/format/webp&token=preview" {
		t.Fatalf("expected preview url, got %#v", got)
	}
	if len(requests) != 2 || requests[0].Query != "" || requests[1].Query != qiniuPreviewQuery {
		t.Fatalf("expected original and preview sign requests, got %#v", requests)
	}
}

func qboxTestCredentials() *qbox.Mac {
	return qbox.NewMac("test-ak", "test-sk")
}

func decodeQiniuPolicy(value string) ([]byte, error) {
	if raw, err := base64.URLEncoding.DecodeString(value); err == nil {
		return raw, nil
	}
	return base64.RawURLEncoding.DecodeString(value)
}
