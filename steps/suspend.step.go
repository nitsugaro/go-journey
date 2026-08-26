package steps

import (
	"strings"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-ndb"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/crypto"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

type SuspendFlow struct {
	BasicStep

	_   struct{} `description:"Suspend Journey Flow for a while. Reanude with a resume ID."`
	Exp string   `json:"exp" description:"Seconds expiration."`
	URI string   `json:"uri" description:"Additional data information to return to the client." default:"{{resume_id}}"`
}

func (uns *SuspendFlow) GetStepType() string {
	return types.SuspendFlowStep
}

func (uns *SuspendFlow) Execute(journeyTransaction *types.JourneyTransaction, config goutils.TreeMapImpl) (string, error) {
	closedCtx := journeyTransaction.State.GetClosedCtx()
	suspendKey := env.GetSuspendJourneyKey()
	if closedCtx.Get(suspendKey+".step_id").AsStringOr("") == journeyTransaction.CurrentStepID {
		closedCtx.Delete(suspendKey)
		return "true", nil
	}

	tempCtx := journeyTransaction.State.GetTempCtx()

	id, err := crypto.GetRandBytes(16)
	if err != nil {
		return "", err
	}

	resumeID := encoding.EncodeBase64URL(id)
	resumeURI := strings.Replace(config.Get("uri").AsStringOr("{{resume_id}}"), "{{resume_id}}", resumeID, 1)
	closedCtx.Set(suspendKey+".journey_id", journeyTransaction.Journey.ID)
	closedCtx.Set(suspendKey+".step_id", journeyTransaction.CurrentStepID)
	tempCtx.Set(suspendKey+".exp", config.Get("exp").AsIntOr(60))
	tempCtx.Set(suspendKey+".resume_id", resumeID)
	tempCtx.Set(suspendKey+".uri", resumeURI)

	message := &inputs.Message{
		ID:       journeyTransaction.CurrentStepID,
		StepType: types.SuspendFlowStep,
	}

	if len(resumeURI) != 0 {
		message.Value = ndb.M{"value": resumeURI}
	}

	journeyTransaction.ClientInputsBuilder.AddMessageInput(message)

	return "true", nil
}

func init() {
	defaultStepRegistry.AddStep(&SuspendFlow{}, map[string]map[string]any{
		".": {"x-category": types.FlowCategory},
	})
}
