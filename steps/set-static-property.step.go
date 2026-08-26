package steps

import (
	"errors"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type SetStaticProperty struct {
	BasicStep

	_ struct{} `description:"Set static-string props in a specific context."`

	Type  string         `json:"type" enum:"ctx,encCtx,closedCtx,tempCtx" required:"true" default:"ctx" description:"Context where property will be saved."`
	Props map[string]any `json:"props" default:"{}" required:"true" additionalProperties.type:"any"`

	Outcome struct {
		True string `json:"true" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *SetStaticProperty) GetStepType() string {
	return types.SetStaticPropertyStep
}

func (uns *SetStaticProperty) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	typeCtx := config.Get("type").AsStringOr("")

	props := map[string]any{}
	err := config.Get("props").AsStruct(&props)
	if err != nil {
		props = map[string]any{}
	}

	ctx := journeyTransaction.State.Get(typeCtx)
	if ctx == nil {
		return "", errors.New("context type is not supported")
	}

	for key, val := range props {
		ctx.Set(key, val)
	}

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&SetStaticProperty{}, map[string]map[string]any{
		".": {
			"x-category": types.ContextCategory,
			"x-order":    []string{"type", "props", "outcome"},
		},
	})
}
