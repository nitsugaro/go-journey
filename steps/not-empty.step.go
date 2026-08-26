package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type NotEmpty struct {
	BasicStep

	_       struct{} `description:"Write a string expression to verify if it's empty."`
	Str     string   `json:"string" required:"true" description:"Use template string to replace a value and verify it."`
	Outcome struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (nt *NotEmpty) GetStepType() string {
	return types.NOT_EMPTY_STEP
}

func (nt *NotEmpty) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	if len(config.Get("string").AsStringOr("")) != 0 {
		return "true", nil
	} else {
		return "false", nil
	}
}

func init() {
	defaultStepRegistry.AddStep(&NotEmpty{}, map[string]map[string]any{
		".": {
			"x-category": types.FlowCategory,
		},
	})
}
