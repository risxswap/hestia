package mysql_test

import (
	"testing"

	"hestia/server/internal/infra/config"
	mysqlinfra "hestia/server/internal/infra/mysql"
)

func TestOpenReturnsNilWhenDSNIsEmpty(t *testing.T) {
	db, err := mysqlinfra.Open(&config.Config{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if db != nil {
		t.Fatalf("expected nil db, got %#v", db)
	}
}

func TestOpenReturnsDBWhenDSNIsConfigured(t *testing.T) {
	db, err := mysqlinfra.Open(&config.Config{
		DatabaseDSN: "user:pass@tcp(localhost:3306)/hestia?parseTime=true",
	})
	if db != nil {
		defer db.Close()
	}

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if db == nil {
		t.Fatalf("expected non-nil db")
	}
}
