package steps

import (
	"errors"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type HTTPFinishResponse struct {
	BasicStep

	_ struct{} `description:"Ends a resource journey with the HTTP response currently written by previous steps."`
}

func (*HTTPFinishResponse) EndJourney() bool {
	return true
}

func (*HTTPFinishResponse) GetStepType() string {
	return types.HTTPFinishResponseStep
}

func (*HTTPFinishResponse) Execute(transaction *types.JourneyTransaction, _ goutils.TreeMapImpl) (string, error) {
	if transaction == nil || transaction.Response == nil {
		return "true", errors.New("response is not available")
	}
	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&HTTPFinishResponse{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-end-journey": true},
	})
}
