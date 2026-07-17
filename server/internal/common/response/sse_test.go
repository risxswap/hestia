package response_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"hestia/server/internal/common/response"
)

func TestWriteSSEWritesNamedEvent(t *testing.T) {
	var builder strings.Builder

	if err := response.WriteSSE(&builder, "status", map[string]string{"text": "处理中"}); err != nil {
		t.Fatalf("write sse: %v", err)
	}

	got := builder.String()
	if !strings.Contains(got, "event: status\n") {
		t.Fatalf("expected status event, got %q", got)
	}
	if !strings.Contains(got, `data: {"text":"处理中"}`+"\n\n") {
		t.Fatalf("expected json data, got %q", got)
	}
}

func TestWriteSSEReturnsShortWrite(t *testing.T) {
	err := response.WriteSSE(shortWriter{}, "delta", map[string]string{"text": "部分建议"})

	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("expected io.ErrShortWrite, got %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return len(p) / 2, nil
}
