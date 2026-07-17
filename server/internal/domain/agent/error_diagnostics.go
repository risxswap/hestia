package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	serverlogger "hestia/server/internal/infra/logger"
)

const (
	maxAgentErrorChainDepth = 8
	maxAgentNodePathRunes   = 160
)

var agentNodePathPattern = regexp.MustCompile(`(?i)node path:\s*\[([^\]\r\n]+)\]`)
var providerHostnamePattern = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)
var agentCredentialRedactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\bapi[\s_-]?key\s*[:=]\s*[^\s,;]+`), "API key=[REDACTED]"},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`), "[REDACTED]"},
	{regexp.MustCompile(`\b[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`), "[REDACTED]"},
}

type agentErrorDiagnostics struct {
	ProviderCode    string
	ModelCode       string
	ProviderHost    string
	AgentTimeoutMS  int64
	LLMTimeoutMS    int64
	ContextDeadline string
	ContextError    string
	ErrorType       string
	ErrorChain      []string
	NodePath        string
	TimeoutSource   string
}

func diagnoseAgentError(ctx context.Context, err error, metadata AdviceRunMetadata) agentErrorDiagnostics {
	diagnostics := agentErrorDiagnostics{
		ProviderCode:   metadata.ProviderCode,
		ModelCode:      metadata.ModelCode,
		ProviderHost:   normalizeProviderHost(metadata.ProviderHost),
		AgentTimeoutMS: metadata.AgentTimeoutMS,
		LLMTimeoutMS:   metadata.LLMTimeoutMS,
		ContextError:   contextErrorCode(ctx),
		ErrorType:      fmt.Sprintf("%T", err),
		ErrorChain:     safeAgentErrorChain(err),
		NodePath:       agentNodePath(err),
		TimeoutSource:  agentTimeoutSource(ctx, err),
	}
	if ctx != nil {
		if deadline, ok := ctx.Deadline(); ok {
			diagnostics.ContextDeadline = deadline.UTC().Format(time.RFC3339Nano)
		}
	}
	return diagnostics
}

func providerHostFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return sanitizedHostname(parsed.Hostname())
}

func normalizeProviderHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		return providerHostFromURL(raw)
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return sanitizedHostname(parsed.Hostname())
}

func sanitizedHostname(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if net.ParseIP(host) != nil || providerHostnamePattern.MatchString(host) {
		return host
	}
	return ""
}

func safeAgentErrorChain(err error) []string {
	chain := make([]string, 0, maxAgentErrorChainDepth)
	for current, depth := err, 0; current != nil && depth < maxAgentErrorChainDepth; current, depth = errors.Unwrap(current), depth+1 {
		chain = append(chain, fmt.Sprintf("%T: %s", current, safeAgentErrorSummary(current)))
	}
	return chain
}

func agentNodePath(err error) string {
	for current, depth := err, 0; current != nil && depth < maxAgentErrorChainDepth; current, depth = errors.Unwrap(current), depth+1 {
		matches := agentNodePathPattern.FindStringSubmatch(current.Error())
		if len(matches) != 2 {
			continue
		}
		path := sanitizeAgentErrorText(strings.TrimSpace(matches[1]))
		runes := []rune(path)
		if len(runes) > maxAgentNodePathRunes {
			path = string(runes[:maxAgentNodePathRunes])
		}
		return path
	}
	return ""
}

func safeAgentErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	return sanitizeAgentErrorText(err.Error())
}

func sanitizeAgentErrorText(value string) string {
	for _, redactor := range agentCredentialRedactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	return serverlogger.SanitizeSummary(value)
}

func contextErrorCode(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return "none"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "canceled"
}

func agentTimeoutSource(ctx context.Context, err error) string {
	if ctx != nil {
		if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			return "agent_context"
		}
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			return "agent_context"
		}
	}
	if hasTransportTimeout(err) {
		return "http_transport"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "llm_client"
	}
	return "unknown"
}

func hasTransportTimeout(err error) bool {
	for current, depth := err, 0; current != nil && depth < maxAgentErrorChainDepth; current, depth = errors.Unwrap(current), depth+1 {
		if current == context.DeadlineExceeded {
			continue
		}
		if netErr, ok := current.(net.Error); ok && netErr.Timeout() {
			return true
		}
	}
	return false
}

func (d agentErrorDiagnostics) logAttrs() []any {
	return []any{
		"provider_code", d.ProviderCode,
		"model_code", d.ModelCode,
		"provider_host", d.ProviderHost,
		"agent_timeout_ms", d.AgentTimeoutMS,
		"llm_timeout_ms", d.LLMTimeoutMS,
		"context_deadline", d.ContextDeadline,
		"context_error", d.ContextError,
		"error_type", d.ErrorType,
		"error_chain", d.ErrorChain,
		"node_path", d.NodePath,
		"timeout_source", d.TimeoutSource,
	}
}
