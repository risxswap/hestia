package id_test

import (
	"strings"
	"testing"

	"hestia/server/internal/common/id"
)

func TestNewPublicIDUsesPrefix(t *testing.T) {
	got := id.NewPublicID("usr")
	if !strings.HasPrefix(got, "usr_") {
		t.Fatalf("expected usr_ prefix, got %q", got)
	}
	if len(got) <= len("usr_") {
		t.Fatalf("expected suffix, got %q", got)
	}
}
