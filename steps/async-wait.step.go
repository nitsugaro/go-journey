package steps

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-journey/utils"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type AsyncWait struct {
	BasicStep

	_       struct{}     `description:"Asynchronous wait to execute a sequence of steps."`
	WaitFor string       `json:"wait_for" enum:"ALL,ANY" default:"ALL" required:"true"`
	Timeout string       `json:"timeout"`
	Steps   []types.Step `json:"steps" required:"true"`
	Outcome struct {
		True  string `json:"true" required:"true" format:"uuid"`
		False string `json:"false" required:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *AsyncWait) GetStepType() string {
	return types.AsyncWaitStep
}

func (uns *AsyncWait) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	var stepsAsync []types.Step
	err := config.Get("steps").AsStruct(&stepsAsync)
	if err != nil {
		return "", types.ErrInvalidStepConfig
	}
	waitFor := config.Get("wait_for").AsStringOr("ALL")
	if waitFor != "ALL" && waitFor != "ANY" {
		return "", types.StepInvalidConfig(journeyTransaction.CurrentStepID, "wait_for must be ALL or ANY")
	}
	for _, step := range stepsAsync {
		if unsafeAsyncStep(step.StepType) {
			return "", types.StepInvalidConfig(journeyTransaction.CurrentStepID, "step "+step.StepType+" cannot run asynchronously")
		}
	}

	timeout := time.Duration(config.Get("timeout").AsIntOr(10)) * time.Second
	parentContext := journeyTransaction.Context
	if parentContext == nil {
		parentContext = context.Background()
	}
	requestContext, cancel := context.WithTimeout(parentContext, timeout)
	defer cancel()
	journeyTransaction.State.SetAllCtx(
		goutils.NewSyncTreeMap(journeyTransaction.State.GetCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetEncryptedCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetClosedCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetTempCtx().AsMap()),
	)

	tasks := goutils.Map(stepsAsync, func(step types.Step, index int) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			currentStep := configuredStep(journeyTransaction, step.StepType)
			if currentStep == nil {
				return "", types.StepNotFound(step.StepType)
			}
			child := cloneTransaction(journeyTransaction)
			child.Context = ctx
			child.CurrentStepID = journeyTransaction.CurrentStepID + ".wait." + strconv.Itoa(index)
			outcome, err := types.ExecuteStepConfig(currentStep, child, step.Config)
			if err == nil && !child.ClientInputsBuilder.IsNewEmpty() {
				return "", types.StepInvalidConfig(step.Name, "asynchronous steps cannot request client input")
			}
			return outcome, err
		}
	})

	ctx := requestContext
	if waitFor == "ALL" {
		if _, taskErrors := utils.PromiseAll(ctx, timeout, nil, tasks); goutils.Some(taskErrors, func(err error, _ int) bool { return err != nil }) {
			return "false", errors.Join(taskErrors...)
		}
	} else {
		if _, err := utils.PromiseAny(ctx, timeout, nil, tasks); err != nil {
			return "false", err
		}
	}

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&AsyncWait{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"wait_for", "timeout", "steps", "outcome"}},
	})
}
