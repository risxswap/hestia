package agent

import (
	"context"
)

const (
	AdviceToolCreateDraft = "create_advice_draft"
	AdviceToolUpdateDraft = "update_advice_draft"
)

type AdviceRunner interface {
	Run(ctx context.Context, input AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error)
}

type AdviceTextDeltaEmitter func(string) error
