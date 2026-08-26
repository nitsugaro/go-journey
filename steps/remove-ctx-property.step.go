package steps

import (
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type RemoveCtxProperty struct {
	BasicStep

	_ struct{} `description:"Remove an specific property from any kind of context with its subproperties."`

	PropertyPath string `json:"property_path" required:"true"  pattern:"^(ctx|encCtx|closedCtx|tempCtx)(\\.\\w+)+$" description:"Path of the property."`
	Outcome      struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *RemoveCtxProperty) GetStepType() string {
	return types.RemoveCtxPropertyStep
}

func (uns *RemoveCtxProperty) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	propertyPath := config.Get("property_path").AsStringOr("")
	ctx, keyPath := journeyTransaction.State.GetCtxPath(propertyPath)
	ctx.Delete(keyPath)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&RemoveCtxProperty{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory},
		"property_path": {
			"x-error": "Value doesn't match pattern: '<CTX>.PATH.TO.KEY.CONTEXT'",
		},
	})
}
