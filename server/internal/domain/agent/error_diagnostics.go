package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
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
	{regexp.MustCompile(`(?i)["']?api[\s_-]?key["']?\s*[:=]\s*["']?[^"'\s,;}]+["']?`), "API key=[REDACTED]"},
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
	value = stripAgentErrorGoQuotedJSON(value)
	value = stripAgentErrorJSONBodies(value)
	for _, redactor := range agentCredentialRedactors {
		value = redactor.pattern.ReplaceAllString(value, redactor.replacement)
	}
	return serverlogger.SanitizeSummary(value)
}

func stripAgentErrorGoQuotedJSON(value string) string {
	var builder strings.Builder
	for offset := 0; offset < len(value); {
		if value[offset] != '"' {
			builder.WriteByte(value[offset])
			offset++
			continue
		}
		end := goQuotedStringEnd(value, offset)
		if end <= offset {
			builder.WriteByte(value[offset])
			offset++
			continue
		}
		literal := value[offset:end]
		unquoted, err := strconv.Unquote(literal)
		trimmed := strings.TrimSpace(unquoted)
		if err == nil && len(trimmed) > 1 && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid([]byte(trimmed)) {
			builder.WriteString("[JSON]")
		} else {
			builder.WriteString(literal)
		}
		offset = end
	}
	return builder.String()
}

func goQuotedStringEnd(value string, start int) int {
	escaped := false
	for i := start + 1; i < len(value); i++ {
		if escaped {
			escaped = false
			continue
		}
		if value[i] == '\\' {
			escaped = true
			continue
		}
		if value[i] == '"' {
			return i + 1
		}
	}
	return -1
}

func stripAgentErrorJSONBodies(value string) string {
	var builder strings.Builder
	for offset := 0; offset < len(value); {
		if value[offset] != '{' && value[offset] != '[' {
			builder.WriteByte(value[offset])
			offset++
			continue
		}
		end := balancedJSONEnd(value, offset)
		if end <= offset || !json.Valid([]byte(value[offset:end])) {
			builder.WriteByte(value[offset])
			offset++
			continue
		}
		builder.WriteString("[JSON]")
		offset = end
	}
	return builder.String()
}

func balancedJSONEnd(value string, start int) int {
	stack := make([]byte, 0, 4)
	inString := false
	escaped := false
	for i := start; i < len(value); i++ {
		character := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
				continue
			}
			if character == '"' {
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, character)
		case '}', ']':
			if len(stack) == 0 || !matchingJSONBrackets(stack[len(stack)-1], character) {
				return -1
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func matchingJSONBrackets(open, close byte) bool {
	return open == '{' && close == '}' || open == '[' && close == ']'
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
