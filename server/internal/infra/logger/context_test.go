package logger

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), " req_test_123 ")
	if got := RequestID(ctx); got != "req_test_123" {
		t.Fatalf("expected request id req_test_123, got %q", got)
	}
	if got := RequestID(nil); got != "" {
		t.Fatalf("expected empty request id for nil context, got %q", got)
	}
}

func TestSanitizeSummaryRedactsSensitiveValues(t *testing.T) {
	raw := "联系 test@example.com 13800138000 https://example.test/a?token=secret Bearer abc api_key=xyz"
	got := SanitizeSummary(raw)
	for _, leaked := range []string{"test@example.com", "13800138000", "example.test", "abc", "xyz"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected summary to redact %q, got %q", leaked, got)
		}
	}
	for _, marker := range []string{"[EMAIL]", "[PHONE]", "[URL]", "[REDACTED]"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("expected summary to contain %q, got %q", marker, got)
		}
	}
}

func TestSanitizeSummaryCollapsesWhitespaceAndTruncatesRunes(t *testing.T) {
	got := SanitizeSummary("  第一行\n\t第二行  " + strings.Repeat("好", 300))
	if !strings.HasPrefix(got, "第一行 第二行") {
		t.Fatalf("expected collapsed whitespace, got %q", got)
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Fatalf("expected truncation marker, got %q", got)
	}
	if len([]rune(got)) > summaryMaxRunes+len([]rune("...[truncated]")) {
		t.Fatalf("expected bounded summary, got %d runes", len([]rune(got)))
	}
}

func TestErrorSummaryUsesSanitizer(t *testing.T) {
	got := ErrorSummary(errors.New("provider failed for Bearer secret-token"))
	if strings.Contains(got, "secret-token") {
		t.Fatalf("expected error summary to redact token, got %q", got)
	}
	if got := ErrorSummary(nil); got != "" {
		t.Fatalf("expected empty summary for nil error, got %q", got)
	}
}
