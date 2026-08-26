package steps

import (
	"github.com/PaesslerAG/gval"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type IfExpression struct {
	BasicStep

	_          struct{} `description:"IfExpression to execute a conditional expression."`
	Expression string   `json:"expression" required:"true" description:"Go Expression to get a boolean value."`
	Outcome    struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *IfExpression) GetStepType() string {
	return types.IfExpressionStep
}

func (uns *IfExpression) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	exp := config.Get("expression").AsStringOr("")
	val, err := gval.Evaluate(exp, journeyTransaction.ExpressionBindings())

	if val == true {
		return "true", nil
	} else {
		return "false", err
	}
}

func init() {
	defaultStepRegistry.AddStep(&IfExpression{}, map[string]map[string]any{
		".": {
			"x-category": types.FlowCategory,
		},
		"expression": {
			"x-type": "scriptable",
		},
	})
}
