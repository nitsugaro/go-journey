package steps

import (
	"strconv"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type ExtendExp struct {
	BasicStep

	_       struct{} `description:"Extend Journey expiration."`
	Minutes string   `json:"minutes" default:"3" description:"Minutes to be added/substract to Journey. Default 3 minutes."`
	Outcome struct {
		True string `json:"true" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (uns *ExtendExp) GetStepType() string {
	return types.ExtendExpStep
}

func (uns *ExtendExp) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	minutes, err := strconv.ParseFloat(config.Get("minutes").AsStringOr("3"), 32)
	if err != nil {
		return "", err
	}

	journeyTransaction.State.GetCtx().Set(env.GetExtendJourneyExpKey(), minutes)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&ExtendExp{}, map[string]map[string]any{
		".": {"x-category": types.ContextCategory},
	})
}
