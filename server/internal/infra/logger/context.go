package logger

import (
	"context"
	"regexp"
	"strings"
)

const summaryMaxRunes = 240

type requestIDContextKey struct{}

var summaryRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)https?://[^\s]+`), "[URL]"},
	{regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`), "[EMAIL]"},
	{regexp.MustCompile(`(?:\+?86[-\s]?)?1[3-9]\d{9}`), "[PHONE]"},
	{regexp.MustCompile(`(?i)\bbearer\s+[^\s,;]+`), "Bearer [REDACTED]"},
	{regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|token)\s*[:=]\s*[^\s,;]+`), "$1=[REDACTED]"},
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDContextKey{}, strings.TrimSpace(requestID))
}

func RequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return strings.TrimSpace(requestID)
}

func SanitizeSummary(value string) string {
	summary := strings.Join(strings.Fields(value), " ")
	for _, redactor := range summaryRedactors {
		summary = redactor.pattern.ReplaceAllString(summary, redactor.replacement)
	}
	runes := []rune(summary)
	if len(runes) <= summaryMaxRunes {
		return summary
	}
	return string(runes[:summaryMaxRunes]) + "...[truncated]"
}

func ErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeSummary(err.Error())
}
