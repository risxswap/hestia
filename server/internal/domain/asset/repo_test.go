package asset

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestMySQLRepositoryCreateWritesFilesTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO files")).
		WillReturnResult(sqlmock.NewResult(42, 1))

	created, err := repo.Create(context.Background(), Asset{
		PublicID:     "ast_abcdefghijklmnopqrstuvwxyz",
		OwnerUserID:  12,
		Bucket:       "hestia-dev",
		ObjectKey:    "users/12/wardrobe/ast_abcdefghijklmnopqrstuvwxyz.jpg",
		MimeType:     "image/jpeg",
		FileSize:     2048,
		AssetType:    "wardrobe_item_photo",
		Source:       SourceMiniappUpload,
		Status:       StatusActive,
		ReviewStatus: ReviewStatusPending,
		Metadata:     map[string]any{},
	})
	if err != nil {
		t.Fatalf("create file row: %v", err)
	}
	if created.ID != 42 {
		t.Fatalf("expected inserted id 42, got %d", created.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMySQLRepositoryFindByPublicIDReadsFilesTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer db.Close()
	repo := NewMySQLRepositoryWithExt(sqlx.NewDb(db, "sqlmock"))

	mock.ExpectQuery(regexp.QuoteMeta("FROM files")).
		WithArgs("ast_abcdefghijklmnopqrstuvwxyz").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "owner_user_id", "bucket", "object_key", "mime_type", "file_size",
			"width", "height", "asset_type", "source", "status", "review_status", "metadata_json",
		}).AddRow(
			42, "ast_abcdefghijklmnopqrstuvwxyz", 12, "hestia-dev",
			"users/12/wardrobe/ast_abcdefghijklmnopqrstuvwxyz.jpg", "image/jpeg", 2048,
			nil, nil, "wardrobe_item_photo", SourceMiniappUpload, StatusActive, ReviewStatusPending, `{}`,
		))

	item, err := repo.FindByPublicID(context.Background(), "ast_abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		t.Fatalf("find file row: %v", err)
	}
	if item.ID != 42 || item.PublicID != "ast_abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("unexpected file row: %#v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
