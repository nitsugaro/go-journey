package steps

import (
	"fmt"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Retry struct {
	BasicStep

	_           struct{} `description:"Counts visits and routes to retry until the configured attempt limit is reached."`
	MaxAttempts int      `json:"max_attempts" required:"true" minimum:"1"`
	Context     string   `json:"context" enum:"ctx,encCtx,closedCtx,tempCtx" default:"ctx"`
	Counter     string   `json:"counter" required:"true" minLength:"1" description:"Property path where the incremented attempt count is stored."`
	Outcome     struct {
		Retry     string `json:"retry" required:"true" format:"uuid"`
		Exhausted string `json:"exhausted" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (*Retry) GetStepType() string { return types.RetryStep }

func (*Retry) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	maxAttempts := config.Get("max_attempts").AsIntOr(0)
	if maxAttempts < 1 {
		return "", types.ErrInvalidStepConfig
	}
	contextMap := transaction.State.Get(config.Get("context").AsStringOr(types.CtxKey))
	if contextMap == nil {
		return "", fmt.Errorf("retry context is not available")
	}
	counter := config.Get("counter").AsStringOr("")
	if counter == "" {
		return "", types.ErrInvalidStepConfig
	}
	attempts := contextMap.Get(counter).AsIntOr(0) + 1
	contextMap.Set(counter, attempts)
	if attempts < maxAttempts {
		return "retry", nil
	}
	return "exhausted", nil
}

func init() {
	defaultStepRegistry.AddStep(&Retry{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"max_attempts", "context", "counter", "outcome"}},
	})
}
