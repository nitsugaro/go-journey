package steps

import (
	"errors"

	"github.com/PaesslerAG/gval"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type SetCtxProperty struct {
	BasicStep

	_ struct{} `description:"Evaluates a dynamic expression and stores its result in a selected journey context."`

	Type       string `json:"type" enum:"ctx,encCtx,closedCtx,tempCtx" required:"true" default:"ctx" description:"Context where property will be saved."`
	Key        string `json:"key" required:"true" description:"Name of property to be save as key."`
	Expression string `json:"expression" required:"true" description:"Go Expression to be save as value."`
	Outcome    struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *SetCtxProperty) GetStepType() string {
	return types.SetCtxPropertyStep
}

func (uns *SetCtxProperty) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	typeCtx := config.Get("type").AsStringOr("")
	key := config.Get("key").AsStringOr("")
	exp, err := config.Get("expression").AsString()
	if err != nil {
		return "false", nil
	}

	ctx := journeyTransaction.State.Get(typeCtx)
	if ctx == nil {
		return "", errors.New("context type is not supported")
	}

	val, err := gval.Evaluate(exp, journeyTransaction.ExpressionBindings())
	if err != nil {
		return "false", nil
	} else {
		ctx.Set(key, val)
		return "true", nil
	}
}

func init() {
	defaultStepRegistry.AddStep(&SetCtxProperty{}, map[string]map[string]any{
		"expression": {
			"x-type": "scriptable",
		},
		".": {
			"x-category": types.ContextCategory,
			"x-order":    []string{"type", "key", "expression", "outcome"},
		},
	})
}
