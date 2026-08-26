package steps

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

var BLACKLIST_STEPS_CHAIN = []string{types.ChainStep, types.SubJourneyStep, types.AsyncWaitStep, types.AsyncExecStep}

type Chain struct {
	BasicStep

	_       struct{}          `description:"Chain to execute a sequence of steps."`
	Steps   []types.Step      `json:"steps" required:"true"`
	Outcome map[string]string `json:"outcome" required:"true"`
}

func (cs *Chain) VerifyConfig(stepName string, config goutils.TreeMapImpl) error {
	var chainConfig Chain
	if err := config.AsStruct(&chainConfig); err != nil {
		return err
	}

	for _, chainStep := range chainConfig.Steps {
		if slices.Contains(BLACKLIST_STEPS_CHAIN, chainStep.StepType) {
			return types.StepInvalidConfig(stepName, fmt.Sprintf("%s steps can't contain '%s' steps", types.ChainStep, chainStep.StepType))
		}

		if defaultStepRegistry.GetStep(chainStep.StepType) != nil {
			if err := defaultStepRegistry.ValidateStep(&chainStep); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cs *Chain) GetStepType() string {
	return types.ChainStep
}

func (uns *Chain) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	var stepsChain []types.Step
	err := config.Get("steps").AsStruct(&stepsChain)
	if err != nil {
		return "", types.ErrInvalidStepConfig
	}

	originalStepID := journeyTransaction.CurrentStepID
	journeyTransaction.ChainStepID = originalStepID
	defer func() {
		journeyTransaction.ChainStepID = ""
		journeyTransaction.CurrentStepID = originalStepID
	}()
	currentOutcome := ""
	for index, stepChain := range stepsChain {
		currentStep := configuredStep(journeyTransaction, stepChain.StepType)
		if currentStep == nil {
			return "", types.StepNotFound(stepChain.StepType)
		}

		journeyTransaction.CurrentStepID = journeyTransaction.Journey.ID + strconv.FormatInt(int64(index), 10)
		outcome, err := types.ExecuteStepConfig(currentStep, journeyTransaction, stepChain.Config)
		if err != nil {
			return "", err
		}

		if outcome != "" {
			currentOutcome = outcome
		}
	}

	if !journeyTransaction.ClientInputsBuilder.IsNewEmpty() {
		return "", nil
	}

	journeyTransaction.State.GetClosedCtx().Delete(originalStepID)

	return currentOutcome, nil
}

func init() {
	defaultStepRegistry.AddStep(&Chain{}, map[string]map[string]any{
		".":       {"x-category": types.FlowCategory, "x-order": []string{"steps", "outcome"}},
		"outcome": {"x-dynamic-outcome": true},
	})
}
