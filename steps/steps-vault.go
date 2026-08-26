package steps

import (
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type BasicStep struct {
}

func (bs *BasicStep) EndJourney() bool {
	return false
}

func (bs *BasicStep) VerifyConfig(_ string, _ goutils.TreeMapImpl) error {
	return nil
}

func configuredStep(transaction *types.JourneyTransaction, stepType string) types.IStep {
	if transaction.Steps == nil {
		return nil
	}
	return transaction.Steps.GetStep(stepType)
}

func cloneTransaction(transaction *types.JourneyTransaction) *types.JourneyTransaction {
	childState := types.NewJourneyState()
	childState.Exp = transaction.State.Exp
	childState.SetID(transaction.State.GetID())
	childState.SetRealm(transaction.State.GetRealm())
	childState.SetAllCtx(
		transaction.State.GetCtx(),
		transaction.State.GetEncryptedCtx(),
		transaction.State.GetClosedCtx(),
		transaction.State.GetTempCtx(),
	)
	return &types.JourneyTransaction{
		Context:              transaction.Context,
		Request:              transaction.Request,
		CacheManager:         transaction.CacheManager,
		Journey:              transaction.Journey,
		CurrentStepID:        transaction.CurrentStepID,
		ChainStepID:          transaction.ChainStepID,
		ClientInputsBuilder:  inputs.NewClientInputBuilder(nil, transaction.ClientInputsBuilder.GetCtxManager(), transaction.CacheManager),
		Response:             transaction.Response,
		State:                childState,
		Steps:                transaction.Steps,
		Payload:              transaction.Payload,
		OnAsyncError:         transaction.OnAsyncError,
		PlaceholderResolvers: transaction.PlaceholderResolvers,
		Observer:             transaction.Observer,
		InteractionState:     transaction.InteractionState.Share(),
	}
}

func unsafeAsyncStep(stepType string) bool {
	switch stepType {
	case types.AsyncExecStep, types.AsyncWaitStep, types.ChainStep, types.SubJourneyStep,
		types.SuccessStep, types.FailureStep, types.SuspendFlowStep, types.FormStep,
		types.HTTPResponseStep, types.HTTPFinishResponseStep, types.HTTPProxyStep, types.SetCookieStep, types.ReadCookieStep, types.ChoiceStep, types.MetadataStep, types.WaitUntilStep, types.RetryStep, types.ScriptStep:
		return true
	default:
		return false
	}
}

var defaultStepRegistry = types.NewStepRegistry()

func GetDefaultStepRegistry() *types.Steps {
	return defaultStepRegistry
}
