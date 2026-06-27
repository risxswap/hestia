package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"hestia/server/internal/infra/config"
)

func TestLoadUsesDefaultPorts(t *testing.T) {
	unsetenv(t, "SERVER_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Port != "8080" {
		t.Fatalf("expected port 8080, got %q", cfg.Port)
	}
}

func TestLoadUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("SERVER_PORT", "18080")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Port != "18080" {
		t.Fatalf("expected port 18080, got %q", cfg.Port)
	}
}

func TestConfigExposesOnlyRuntimeFields(t *testing.T) {
	cfgType := reflect.TypeOf(config.Config{})
	expected := []string{
		"AppEnv",
		"Port",
		"DatabaseDSN",
		"RedisAddr",
		"RedisPassword",
		"RedisDB",
		"LLMProvider",
		"QiniuAccessKey",
		"QiniuSecretKey",
		"QiniuBucket",
		"QiniuUploadHost",
		"QiniuPrivateDomain",
		"QiniuUploadTokenTTLSeconds",
		"QiniuDownloadURLTTLSeconds",
	}

	if cfgType.NumField() != len(expected) {
		t.Fatalf("expected %d config fields, got %d", len(expected), cfgType.NumField())
	}
	for _, name := range expected {
		if _, ok := cfgType.FieldByName(name); !ok {
			t.Fatalf("expected config field %s", name)
		}
	}
}

func TestLoadUsesConfigTOML(t *testing.T) {
	unsetenv(t, "DATABASE_DSN")
	unsetenv(t, "REDIS_ADDR")
	unsetenv(t, "REDIS_PASSWORD")
	unsetenv(t, "REDIS_DB")
	unsetenv(t, "QINIU_ACCESS_KEY")
	unsetenv(t, "QINIU_SECRET_KEY")
	unsetenv(t, "QINIU_BUCKET")
	unsetenv(t, "QINIU_UPLOAD_HOST")
	unsetenv(t, "QINIU_PRIVATE_DOMAIN")
	unsetenv(t, "QINIU_UPLOAD_TOKEN_TTL_SECONDS")
	unsetenv(t, "QINIU_DOWNLOAD_URL_TTL_SECONDS")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
[mysql]
host = "127.0.0.1"
port = 3307
user = "app"
password = "secret"
database = "hestia_test"

[redis]
host = "127.0.0.2"
port = 6380
password = "redis-secret"
database = 3

[qiniu]
access_key = "ak"
secret_key = "sk"
bucket = "private-assets"
upload_host = "https://upload.example.test"
private_domain = "private.example.test"
upload_token_ttl_seconds = 1800
download_url_ttl_seconds = 600
`), 0o600)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	expectedDSN := "app:secret@tcp(127.0.0.1:3307)/hestia_test?charset=utf8mb4&parseTime=true&loc=Local"
	if cfg.DatabaseDSN != expectedDSN {
		t.Fatalf("expected database dsn %q, got %q", expectedDSN, cfg.DatabaseDSN)
	}
	if cfg.RedisAddr != "127.0.0.2:6380" {
		t.Fatalf("expected redis addr 127.0.0.2:6380, got %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "redis-secret" {
		t.Fatalf("expected redis password from config file")
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("expected redis db 3, got %d", cfg.RedisDB)
	}
	if cfg.QiniuAccessKey != "ak" || cfg.QiniuSecretKey != "sk" {
		t.Fatalf("expected qiniu credentials from config file")
	}
	if cfg.QiniuBucket != "private-assets" {
		t.Fatalf("expected qiniu bucket private-assets, got %q", cfg.QiniuBucket)
	}
	if cfg.QiniuUploadHost != "https://upload.example.test" {
		t.Fatalf("expected qiniu upload host, got %q", cfg.QiniuUploadHost)
	}
	if cfg.QiniuPrivateDomain != "private.example.test" {
		t.Fatalf("expected qiniu private domain, got %q", cfg.QiniuPrivateDomain)
	}
	if cfg.QiniuUploadTokenTTLSeconds != 1800 {
		t.Fatalf("expected qiniu upload ttl 1800, got %d", cfg.QiniuUploadTokenTTLSeconds)
	}
	if cfg.QiniuDownloadURLTTLSeconds != 600 {
		t.Fatalf("expected qiniu download ttl 600, got %d", cfg.QiniuDownloadURLTTLSeconds)
	}
}

func TestLoadUsesQiniuEnvironmentOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`
[qiniu]
access_key = "toml-ak"
secret_key = "toml-sk"
bucket = "toml-bucket"
upload_host = "https://toml-upload.example.test"
private_domain = "toml-private.example.test"
upload_token_ttl_seconds = 1800
download_url_ttl_seconds = 600
`), 0o600)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("QINIU_ACCESS_KEY", "env-ak")
	t.Setenv("QINIU_SECRET_KEY", "env-sk")
	t.Setenv("QINIU_BUCKET", "env-bucket")
	t.Setenv("QINIU_UPLOAD_HOST", "https://env-upload.example.test")
	t.Setenv("QINIU_PRIVATE_DOMAIN", "env-private.example.test")
	t.Setenv("QINIU_UPLOAD_TOKEN_TTL_SECONDS", "7200")
	t.Setenv("QINIU_DOWNLOAD_URL_TTL_SECONDS", "1200")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.QiniuAccessKey != "env-ak" || cfg.QiniuSecretKey != "env-sk" {
		t.Fatalf("expected qiniu credentials from env, got %q/%q", cfg.QiniuAccessKey, cfg.QiniuSecretKey)
	}
	if cfg.QiniuBucket != "env-bucket" {
		t.Fatalf("expected qiniu bucket from env, got %q", cfg.QiniuBucket)
	}
	if cfg.QiniuUploadHost != "https://env-upload.example.test" {
		t.Fatalf("expected qiniu upload host from env, got %q", cfg.QiniuUploadHost)
	}
	if cfg.QiniuPrivateDomain != "env-private.example.test" {
		t.Fatalf("expected qiniu private domain from env, got %q", cfg.QiniuPrivateDomain)
	}
	if cfg.QiniuUploadTokenTTLSeconds != 7200 {
		t.Fatalf("expected qiniu upload ttl from env, got %d", cfg.QiniuUploadTokenTTLSeconds)
	}
	if cfg.QiniuDownloadURLTTLSeconds != 1200 {
		t.Fatalf("expected qiniu download ttl from env, got %d", cfg.QiniuDownloadURLTTLSeconds)
	}
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	previous, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}
