package llm

import "context"

type Client interface {
	Complete(ctx context.Context, input string) (string, error)
}
