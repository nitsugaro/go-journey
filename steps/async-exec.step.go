package steps

import (
	"strconv"
	"sync"

	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type AsyncExec struct {
	BasicStep

	_         struct{}     `description:"Executes a list of steps asynchronous."`
	RunMethod string       `json:"run_method" enum:"SEQUENTLY,CONCURRENT" default:"SEQUENTLY" description:"How steps will be executed."`
	Steps     []types.Step `json:"steps" required:"true"`
	Outcome   struct {
		True string `json:"true" format:"uuid"`
	} `json:"outcome" required:"true"`
}

func (cs *AsyncExec) GetStepType() string {
	return types.AsyncExecStep
}

func (uns *AsyncExec) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	var stepsAsync []types.Step
	err := config.Get("steps").AsStruct(&stepsAsync)
	if err != nil {
		return "", types.ErrInvalidStepConfig
	}
	runMethod := config.Get("run_method").AsStringOr("SEQUENTLY")
	if runMethod != "SEQUENTLY" && runMethod != "CONCURRENT" {
		return "", types.StepInvalidConfig(journeyTransaction.CurrentStepID, "run_method must be SEQUENTLY or CONCURRENT")
	}
	for index := range stepsAsync {
		step := &stepsAsync[index]
		if unsafeAsyncStep(step.StepType) {
			return "", types.StepInvalidConfig(journeyTransaction.CurrentStepID, "step "+step.StepType+" cannot run asynchronously")
		}
	}

	journeyTransaction.State.SetAllCtx(
		goutils.NewSyncTreeMap(journeyTransaction.State.GetCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetEncryptedCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetClosedCtx().AsMap()),
		goutils.NewSyncTreeMap(journeyTransaction.State.GetTempCtx().AsMap()),
	)

	baseTransaction := cloneTransaction(journeyTransaction)
	run := func(step *types.Step, index int) {
		currentStep := configuredStep(baseTransaction, step.StepType)
		if currentStep == nil {
			reportAsyncError(baseTransaction, step, types.StepNotFound(step.StepType))
			return
		}
		child := cloneTransaction(baseTransaction)
		child.CurrentStepID = baseTransaction.CurrentStepID + ".async." + strconv.Itoa(index)
		if _, err := types.ExecuteStepConfig(currentStep, child, step.Config); err != nil {
			reportAsyncError(baseTransaction, step, err)
		} else if !child.ClientInputsBuilder.IsNewEmpty() {
			reportAsyncError(baseTransaction, step, types.StepInvalidConfig(step.Name, "asynchronous steps cannot request client input"))
		}
	}

	go func() {
		if runMethod == "SEQUENTLY" {
			for index := range stepsAsync {
				run(&stepsAsync[index], index)
			}
			return
		}
		var group sync.WaitGroup
		group.Add(len(stepsAsync))
		for index := range stepsAsync {
			go func(step *types.Step, index int) {
				defer group.Done()
				run(step, index)
			}(&stepsAsync[index], index)
		}
		group.Wait()
	}()

	return "true", nil
}

func reportAsyncError(transaction *types.JourneyTransaction, step *types.Step, err error) {
	if transaction.OnAsyncError != nil {
		defer func() { _ = recover() }()
		transaction.OnAsyncError(step, err)
	}
}

func init() {
	defaultStepRegistry.AddStep(&AsyncExec{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory, "x-order": []string{"run_method", "steps", "outcome"}},
	})
}
