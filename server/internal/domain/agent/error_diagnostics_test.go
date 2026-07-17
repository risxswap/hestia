package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "transport timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

var _ net.Error = timeoutNetError{}

type wrappedTimeoutNetError struct{ err error }

func (e wrappedTimeoutNetError) Error() string { return "transport deadline: " + e.err.Error() }
func (e wrappedTimeoutNetError) Unwrap() error { return e.err }
func (wrappedTimeoutNetError) Timeout() bool   { return true }
func (wrappedTimeoutNetError) Temporary() bool { return true }

var _ net.Error = wrappedTimeoutNetError{}

func TestDiagnoseAgentErrorTimeoutSource(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		err  error
		want string
	}{
		{
			name: "agent context",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Nanosecond)
			},
			err:  context.DeadlineExceeded,
			want: "agent_context",
		},
		{
			name: "llm client",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Minute)
			},
			err:  fmt.Errorf("model request: %w", context.DeadlineExceeded),
			want: "llm_client",
		},
		{
			name: "http transport",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			err:  timeoutNetError{},
			want: "http_transport",
		},
		{
			name: "http transport wrapping deadline exceeded",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Minute)
			},
			err:  wrappedTimeoutNetError{err: context.DeadlineExceeded},
			want: "http_transport",
		},
		{
			name: "unknown",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			err:  errors.New("bad output"),
			want: "unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.ctx()
			defer cancel()
			if test.want == "agent_context" {
				<-ctx.Done()
			}
			got := diagnoseAgentError(ctx, test.err, AdviceRunMetadata{})
			if got.TimeoutSource != test.want {
				t.Fatalf("timeout source: got %q want %q", got.TimeoutSource, test.want)
			}
		})
	}
}

func TestDiagnoseAgentErrorSanitizesChainAndExtractsNodePath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := fmt.Errorf("outer https://api.example.com/v1?q=secret: %w", errors.New("node path: [node_1, ChatModel] Bearer secret-token api_key=complete-secret"))
	metadata := AdviceRunMetadata{
		ProviderCode:   "qwen",
		ModelCode:      "qwen-plus",
		ProviderHost:   "api.example.com",
		AgentTimeoutMS: 75000,
		LLMTimeoutMS:   60000,
	}

	got := diagnoseAgentError(ctx, err, metadata)
	if got.ProviderHost != "api.example.com" || got.AgentTimeoutMS != 75000 || got.LLMTimeoutMS != 60000 {
		t.Fatalf("unexpected metadata diagnostics: %#v", got)
	}
	if got.NodePath != "node_1, ChatModel" {
		t.Fatalf("node path: got %q", got.NodePath)
	}
	if got.ContextDeadline == "" || got.ContextError != "none" || got.ErrorType == "" {
		t.Fatalf("unexpected context/type diagnostics: %#v", got)
	}
	if len(got.ErrorChain) != 2 {
		t.Fatalf("error chain: %#v", got.ErrorChain)
	}
	joined := fmt.Sprint(got.ErrorChain)
	for _, leaked := range []string{"https://api.example.com/v1?q=secret", "secret-token", "complete-secret"} {
		if contains := stringContains(joined, leaked); contains {
			t.Fatalf("leaked %q in %s", leaked, joined)
		}
	}
}

func TestDiagnoseAgentErrorExtractsNodePathAfterLongErrorPrefix(t *testing.T) {
	err := errors.New(strings.Repeat("长前缀", 100) + " node path: [node_1, ChatModel]")

	got := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{})
	if got.NodePath != "node_1, ChatModel" {
		t.Fatalf("node path after long prefix: got %q", got.NodePath)
	}
}

func TestDiagnoseAgentErrorRedactsAdditionalCredentialForms(t *testing.T) {
	err := errors.New(`provider rejected API key: complete-secret "api_key":"quoted-secret" sk-abcdefghijklmnopqrstuvwxyz JWT eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature`)

	got := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{})
	joined := strings.Join(got.ErrorChain, " ")
	for _, leaked := range []string{
		"complete-secret",
		"quoted-secret",
		"sk-abcdefghijklmnopqrstuvwxyz",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
	} {
		if strings.Contains(joined, leaked) {
			t.Fatalf("leaked credential %q in %s", leaked, joined)
		}
	}
}

func TestAgentErrorSummaryRedactsCredentialAcrossTruncationBoundary(t *testing.T) {
	err := errors.New(strings.Repeat("x", 235) + " sk-abcdefghijklmnopqrstuvwxyz")

	topLevel := safeAgentErrorSummary(err)
	if strings.Contains(topLevel, "sk-") {
		t.Fatalf("top-level error leaked credential prefix: %q", topLevel)
	}
	diagnostics := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{})
	if joined := strings.Join(diagnostics.ErrorChain, " "); strings.Contains(joined, "sk-") {
		t.Fatalf("error_chain leaked credential prefix: %q", joined)
	}
}

func TestAgentErrorSummaryStripsNestedJSONRequestBodies(t *testing.T) {
	inner := errors.New(`upstream response [{"content":"用户敏感提示词"}]`)
	err := fmt.Errorf(`provider rejected request body={"api_key":"complete-secret","messages":[{"content":"用户敏感提示词"}]}: %w`, inner)

	topLevel := safeAgentErrorSummary(err)
	if !strings.Contains(topLevel, "provider rejected request body=[JSON]") {
		t.Fatalf("top-level error should preserve diagnostic prefix and replace JSON: %q", topLevel)
	}
	diagnostics := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{})
	joined := strings.Join(diagnostics.ErrorChain, " ")
	for _, output := range []string{topLevel, joined} {
		for _, leaked := range []string{"complete-secret", "用户敏感提示词", `"messages"`, `"content"`} {
			if strings.Contains(output, leaked) {
				t.Fatalf("leaked JSON request content %q in %q", leaked, output)
			}
		}
	}
}

func TestAgentErrorSummaryStripsGoQuotedJSONRequestBodies(t *testing.T) {
	body := `{"api_key":"quoted-body-secret","messages":[{"content":"用户敏感提示词"}]}`
	inner := fmt.Errorf("upstream message=%q request body=%q", "normal diagnostic", body)
	err := fmt.Errorf("provider request failed: %w", inner)

	topLevel := safeAgentErrorSummary(err)
	if !strings.Contains(topLevel, "request body=[JSON]") {
		t.Fatalf("top-level error should replace Go-quoted JSON: %q", topLevel)
	}
	if !strings.Contains(topLevel, `message="normal diagnostic"`) {
		t.Fatalf("top-level error should preserve ordinary quoted text: %q", topLevel)
	}
	diagnostics := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{})
	joined := strings.Join(diagnostics.ErrorChain, " ")
	for _, output := range []string{topLevel, joined} {
		for _, leaked := range []string{"quoted-body-secret", "用户敏感提示词", `\"messages\"`, `\"content\"`} {
			if strings.Contains(output, leaked) {
				t.Fatalf("leaked Go-quoted JSON content %q in %q", leaked, output)
			}
		}
	}
}

func TestProviderHostFromURLReturnsHostnameOnly(t *testing.T) {
	got := providerHostFromURL("https://user:pass@api.example.com:8443/v1?q=secret")
	if got != "api.example.com" {
		t.Fatalf("provider host: got %q", got)
	}
	if got := providerHostFromURL("not a url"); got != "" {
		t.Fatalf("invalid provider URL should be empty, got %q", got)
	}
}

func TestDiagnoseAgentErrorNormalizesProviderHostAndLimitsChain(t *testing.T) {
	err := errors.New("root")
	for i := 0; i < 10; i++ {
		err = fmt.Errorf("layer %d: %w", i, err)
	}
	got := diagnoseAgentError(context.Background(), err, AdviceRunMetadata{
		ProviderHost: "https://user:pass@api.example.com:8443/v1?q=secret",
	})
	if got.ProviderHost != "api.example.com" {
		t.Fatalf("provider host must be hostname only, got %q", got.ProviderHost)
	}
	if len(got.ErrorChain) != maxAgentErrorChainDepth {
		t.Fatalf("error chain length: got %d want %d", len(got.ErrorChain), maxAgentErrorChainDepth)
	}
}

func stringContains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
