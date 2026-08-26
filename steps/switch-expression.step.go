package steps

import (
	"github.com/PaesslerAG/gval"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type ExpressionConfig struct {
	Name string   `json:"name" minLength:"1" required:"true"`
	Exp  string   `json:"exp" minLength:"1" required:"true"`
	_    struct{} `additionalProperties:"false"`
}

type SwitchExpression struct {
	BasicStep

	_           struct{}           `description:"SwitchExpression to execute a conditional expression."`
	Expressions []ExpressionConfig `json:"expressions" additionalProperties.items:"false" required:"true"`
	Outcome     map[string]string  `json:"outcome" required:"true"`
}

func (cs *SwitchExpression) GetStepType() string {
	return types.SwitchExpressionStep
}

func (uns *SwitchExpression) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	expressions, err := config.Get("expressions").AsSlice()
	if err != nil {
		return "", nil
	}

	bindings := journeyTransaction.ExpressionBindings()
	for _, item := range expressions {
		exp := item.Get("exp").AsStringOr("")
		val, err := gval.Evaluate(exp, bindings)

		if err != nil {
			return "", err
		}

		if val == true {
			return item.Get("name").AsStringOr(""), nil
		}
	}

	return "", types.StepInvalidOutcome(journeyTransaction.CurrentStepID, "")
}

func init() {
	defaultStepRegistry.AddStep(&SwitchExpression{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory},
		"expressions": {
			"exp": map[string]any{
				"x-type": "scriptable",
			},
		},
	})
}
