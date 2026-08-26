package steps

import (
	"errors"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type End struct {
	BasicStep

	_      struct{} `description:"Ends a workflow journey and optionally exposes a result to its caller."`
	Result any      `json:"result,omitempty" description:"Optional result returned by workflow/scheduler execution."`
}

func (*End) EndJourney() bool {
	return true
}

func (*End) GetStepType() string {
	return types.EndStep
}

func (*End) Execute(transaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.State == nil {
		return "true", errors.New("journey state is not available")
	}
	hasResult := false
	for _, key := range config.GetKeys() {
		if key == "result" {
			hasResult = true
			break
		}
	}
	result := config.Get("result").AsAnyOr(nil)
	if transaction.State.ExistsTracking() {
		_, stepID := transaction.State.GetTracking()
		transaction.State.GetClosedCtx().Set(stepID, result)
		return "true", nil
	}
	if hasResult {
		transaction.State.SetResult(result)
	}
	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&End{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-end-journey": true, "x-order": []string{"result"}},
	})
}
