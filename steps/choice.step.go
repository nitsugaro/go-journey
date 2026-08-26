package steps

import (
	"sort"

	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type Choice struct {
	BasicStep

	_             struct{}          `description:"Collects user name with client input."`
	DefaultChoice string            `json:"default_choice" description:"Default choice."`
	Outcome       map[string]string `json:"outcome" required:"true"`
}

func (uns *Choice) GetStepType() string {
	return types.ChoiceStep
}

func (uns *Choice) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	Choice := journeyTransaction.ClientInputsBuilder.GetFirstFromID(journeyTransaction.CurrentStepID)
	outcomes, err := config.Get("outcome").AsMap()
	if err != nil {
		return "", err
	}

	keys := make([]string, 0, len(outcomes))
	for k := range outcomes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if Choice == nil {
		journeyTransaction.ClientInputsBuilder.AddStrInput(&inputs.StrInputConfig{
			ID:           journeyTransaction.CurrentStepID,
			StepType:     types.ChoiceStep,
			Value:        keys,
			DefaultValue: config.Get("default_choice").AsStringOr(""),
		})
		return "", nil
	}

	input := journeyTransaction.ClientInputsBuilder.GetFromID(journeyTransaction.CurrentStepID)[0]
	choice := input.GetInput().AsStringOr(config.Get("default_choice").AsStringOr(""))

	return choice, nil
}

func init() {
	defaultStepRegistry.AddStep(&Choice{}, map[string]map[string]any{
		".":       {"x-category": types.FlowCategory, "x-order": []string{"outcome", "default_choice"}},
		"outcome": {"x-dynamic-outcome": true},
	})
}
