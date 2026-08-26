package gojourney

import (
	"errors"
	"strings"
	"time"

	"github.com/nitsugaro/go-journey/env"
	"github.com/nitsugaro/go-journey/inputs"
	"github.com/nitsugaro/go-journey/types"
	goutils "github.com/nitsugaro/go-utils/v2"
	"github.com/nitsugaro/go-utils/v2/crypto"
	"github.com/nitsugaro/go-utils/v2/encoding"
)

func consumeExpirationExtension(ctx goutils.TreeMapImpl) time.Duration {
	if ctx == nil {
		return 0
	}
	return time.Duration(ctx.Delete(env.GetExtendJourneyExpKey()).AsFloatOr(0)) * time.Minute
}

func (jm *journeyManager) GetJourneyState(journeyReq *types.JourneyPayloadReq, isConfidential bool) (*types.JourneyState, *types.JourneyConfiguration, error) {
	if journeyReq == nil {
		return nil, nil, ErrJourneyInvalidPayload
	}
	var (
		journeyState *types.JourneyState
		err          error
		journeyID    string
	)

	hasResume := journeyReq.ResumeID != ""
	hasToken := journeyReq.TokenPresent()
	selectedModes := 0
	if journeyReq.JourneyID != "" {
		selectedModes++
	}
	if hasResume {
		selectedModes++
	}
	if hasToken {
		selectedModes++
	}
	if selectedModes != 1 {
		return nil, nil, JourneyInvalidPayloadErr("exactly one of journey_id, resume_id, or journey_token is required")
	}

	switch {
	case hasResume:
		var ok bool
		journeyState, ok = jm.states.GetAndDelete(journeyReq.ResumeID)
		if !ok {
			return nil, nil, ErrInvalidJourneyToken
		}

		closedCtx := journeyState.GetClosedCtx()
		journeyID = closedCtx.Get(env.GetSuspendJourneyKey() + ".journey_id").AsStringOr("")
		if journeyID == "" || !journeyState.ExistsTracking() {
			return nil, nil, ErrInvalidJourneyToken
		}
		// The suspended state already contains the current tracking frame.
		// Pushing it again would execute the suspended step twice after resume.

	case hasToken:
		// Resume Token
		if journeyState, err = jm.tokens.Validate(journeyReq.Jwt); err != nil {
			return nil, nil, err
		}

		if len(journeyState.TrackingsID) == 0 {
			return nil, nil, ErrInvalidJourneyToken
		}
		tracking := strings.SplitN(journeyState.TrackingsID[len(journeyState.TrackingsID)-1], ":", 2)
		if len(tracking) != 2 || tracking[0] == "" {
			return nil, nil, ErrInvalidJourneyToken
		}
		journeyID = tracking[0]

		prevState, ok := jm.states.GetAndDelete(journeyState.Jti)
		if !ok {
			return nil, nil, ErrInvalidJourneyToken
		}

		journeyState.MergeState(prevState)

	default:
		// New Flow
		journeyID = journeyReq.JourneyID
		journeyState = &types.JourneyState{
			ClosedCtx: goutils.NewTreeMap(),
		}
	}

	journeyState.Init()
	journeyState.SetEncryptionKey(jm.encryptKey)
	if journeyState.EncryptedCtx != "" {
		journeyState.GetEncryptedCtx()
		if journeyState.GetEncryptionError() != nil {
			return nil, nil, ErrInvalidJourneyToken
		}
	}
	journeyConf, err := jm.loadJourney(journeyID)
	if err != nil && !errors.Is(err, ErrJourneyNotFound) {
		return nil, nil, err
	}
	if err != nil || journeyConf == nil || !journeyConf.Active || (journeyConf.Confidential && !isConfidential && !hasToken && !hasResume) {
		return nil, nil, ErrJourneyNotFound
	}

	if realm := journeyReq.GetRealm(); realm != nil && realm.Name != "" && journeyConf.Realm != realm.Name {
		return nil, nil, ErrJourneyNotFound
	}

	if !hasResume && !hasToken {
		realmName := journeyConf.Realm
		if realm := journeyReq.GetRealm(); realm != nil && realm.Name != "" {
			realmName = realm.Name
		}
		journeyState.SetRealm(realmName)
		journeyState.Exp = time.Duration(journeyConf.DefaultExp) * time.Minute
		if len(journeyReq.InitialData) != 0 {
			journeyState.GetCtx().Set("initial_data", journeyReq.InitialData)
		}
		journeyState.PushTracking(journeyID, journeyConf.StartStepID)
	}

	return journeyState, journeyConf, nil
}

func (jm *journeyManager) InvokeJourney(journeyExecute *types.JourneyExecute) (*types.JourneyPayloadReq, *types.JourneyState, error) {
	if journeyExecute == nil || journeyExecute.Payload == nil {
		types.EmitEvent(nil, jm.observer, &types.Event{
			Type:    types.EventFailed,
			Message: "journey invocation rejected",
			Error:   ErrJourneyInvalidPayload,
			Subject: types.EventSubject{Type: "journey"},
		})
		return nil, nil, ErrJourneyInvalidPayload
	}

	journeyPayload := journeyExecute.Payload
	if journeyExecute.Request == nil {
		journeyExecute.Request = types.NewEmptyRequest()
	}
	if journeyExecute.Response == nil {
		journeyExecute.Response = types.NewMemoryResponse()
	}
	journeyState, journeyConf, err := jm.GetJourneyState(journeyPayload, journeyExecute.IsConfidential)
	if err != nil {
		types.EmitEvent(journeyExecute.Context, jm.observer, &types.Event{
			Type:    types.EventFailed,
			Message: "journey invocation rejected",
			Error:   err,
			Subject: types.EventSubject{Type: "journey", ID: journeyPayload.JourneyID},
			Attrs: map[string]any{
				"resume": journeyPayload.ResumeID != "" || journeyPayload.TokenPresent(),
			},
		})
		return nil, nil, err
	}
	if realmName := journeyState.GetRealm(); realmName != "" {
		journeyPayload.SetRealm(&types.Realm{Name: realmName})
	}

	clientInputsCtx := journeyState.GetCtx()
	if journeyConf.EncryptedClientInputs {
		clientInputsCtx = journeyState.GetEncryptedCtx()
		if journeyState.GetEncryptionError() != nil {
			return nil, nil, ErrInvalidJourneyToken
		}
	}

	clientInputsBuilder := inputs.NewClientInputBuilder(journeyPayload.GetClientInputs(), clientInputsCtx, jm.cacheManager)
	if len(journeyPayload.ResumeID) == 0 {
		_, currentStepID := journeyState.GetTracking()
		if clientError := clientInputsBuilder.ValidateProvided(currentStepID); clientError != nil {
			journeyPayload.ClientError = clientError
			if journeyPayload.TokenPresent() && !jm.states.Store(journeyState, journeyState.Exp) {
				return nil, nil, ErrJourneyStateStore
			}
			return journeyPayload, nil, nil
		}
	}

	id, err := crypto.GetRandBytes(16)
	if err != nil {
		return nil, nil, err
	}

	journeyState.SetID(encoding.EncodeBase64(id))

	journeyTransaction := &types.JourneyTransaction{
		Ticks: make(map[string]*struct {
			Count         int64
			LastExecution int64
		}),
		Context:              journeyExecute.Context,
		Request:              journeyExecute.Request,
		Response:             journeyExecute.Response,
		Journey:              journeyConf,
		ClientInputsBuilder:  clientInputsBuilder,
		State:                journeyState,
		CacheManager:         jm.cacheManager,
		Steps:                jm.steps,
		Payload:              journeyPayload,
		OnAsyncError:         journeyExecute.OnAsyncError,
		PlaceholderResolvers: jm.placeholderResolvers,
		Observer:             jm.observer,
	}
	response, state, err := jm.executeJourney(journeyTransaction)
	if response == nil {
		status := types.JourneyExecutionSucceeded
		if err != nil {
			status = types.JourneyExecutionFailed
		}
		jm.emitJourneyExecutionEvent(&types.JourneyExecutionEvent{
			Status:  status,
			Journey: journeyConf,
			State:   state,
			Payload: journeyPayload,
			Error:   err,
		})
	}
	return response, state, err
}

// executeJourney consumes the tracking stack until the journey finishes or a
// step asks the caller for input. Steps control flow by returning an outcome or
// by adding tracking frames (for example, SubJourney); the executor never needs
// to know which concrete step is running.
func (jm *journeyManager) executeJourney(transaction *types.JourneyTransaction) (*types.JourneyPayloadReq, *types.JourneyState, error) {
	for transaction.State.ExistsTracking() {
		journeyID, stepID := transaction.State.PopTracking()
		journey, err := jm.loadJourney(journeyID)
		if err != nil {
			return nil, nil, err
		}

		if stepID == "" {
			stepID = journey.StartStepID
		}

		transaction.Journey = journey
		transaction.CurrentStepID = stepID
		transaction.EmitEvent(&types.Event{
			Type:    types.EventStarted,
			Message: "journey execution started",
			Subject: types.EventSubject{
				Type: "journey", ID: journey.ID, Name: journey.Name,
			},
			Attrs: map[string]any{"realm": transaction.Journey.Realm},
		})
		startedAt := time.Now()

		paused, failed, err := jm.executeSteps(transaction)
		if err != nil {
			transaction.EmitEvent(&types.Event{
				Type:     types.EventFailed,
				Message:  "journey execution failed",
				Duration: time.Since(startedAt),
				Error:    err,
				Subject:  types.EventSubject{Type: "journey", ID: journey.ID, Name: journey.Name},
				Attrs:    journeyEventAttrs(transaction),
			})
			return nil, nil, err
		}
		if failed {
			transaction.EmitEvent(&types.Event{
				Type:     types.EventFailed,
				Message:  "journey execution finished with failure",
				Duration: time.Since(startedAt),
				Error:    ErrJourneyFailure,
				Subject:  types.EventSubject{Type: "journey", ID: journey.ID, Name: journey.Name},
				Attrs:    journeyEventAttrs(transaction),
			})
			return nil, transaction.State, ErrJourneyFailure
		}
		if paused {
			return jm.pauseJourney(transaction)
		}
		transaction.EmitEvent(&types.Event{
			Type:     types.EventFinished,
			Message:  "journey execution finished",
			Duration: time.Since(startedAt),
			Subject:  types.EventSubject{Type: "journey", ID: journey.ID, Name: journey.Name},
			Attrs:    journeyEventAttrs(transaction),
		})
	}

	return nil, transaction.State, nil
}

func (jm *journeyManager) loadJourney(journeyID string) (*types.JourneyConfiguration, error) {
	journey, err := jm.storage.Load(journeyID)
	if err != nil || journey == nil || !journey.Active {
		return nil, ErrJourneyNotFound
	}
	runtimeJourney, err := jm.runtimeJourney(journey)
	if err != nil {
		return nil, err
	}
	return runtimeJourney, nil
}

// executeSteps follows outcomes within one journey frame. An empty outcome
// yields to the tracking stack, allowing a step to transfer control without
// requiring executor changes.
func (jm *journeyManager) executeSteps(transaction *types.JourneyTransaction) (paused bool, failed bool, err error) {
	for {
		if transaction.Journey.Debug {
			transaction.EmitEvent(&types.Event{
				Type:    types.EventDebug,
				Message: "context values",
				Attrs: map[string]any{
					types.CtxKey:       transaction.State.Get(types.CtxKey).AsAnyOr(nil),
					types.EncCtxKey:    transaction.State.Get(types.EncCtxKey).AsAnyOr(nil),
					types.TempCtxKey:   transaction.State.Get(types.TempCtxKey).AsAnyOr(nil),
					types.ClosedCtxKey: transaction.State.Get(types.ClosedCtxKey).AsAnyOr(nil),
				},
			})
		}

		stepConfig, ok := transaction.Journey.Steps[transaction.CurrentStepID]
		if !ok || stepConfig == nil {
			return false, false, ErrStepNotFound
		}

		step := jm.steps.GetStep(stepConfig.StepType)
		if step == nil {
			return false, false, ErrStepNotFound
		}
		stepSubject := types.EventSubject{Type: "step", ID: transaction.CurrentStepID, Name: stepConfig.Name}
		transaction.EmitEvent(&types.Event{
			Type:    types.EventStarted,
			Message: "step execution started",
			Subject: stepSubject,
		})
		stepStartedAt := time.Now()

		var ticks = transaction.Ticks[transaction.CurrentStepID]
		if ticks == nil {
			ticks = &struct {
				Count         int64
				LastExecution int64
			}{}
			transaction.Ticks[transaction.CurrentStepID] = ticks
		}

		if ticks.Count >= env.GetMaxTickCount() && (time.Now().UnixMilli()-ticks.LastExecution) < env.GetMinTickWindowMs() {
			return false, false, JourneyInfiniteLoopErr(stepConfig.Name)
		}

		outcome, err := types.ExecuteStepConfig(step, transaction, stepConfig.Config)
		if err != nil {
			transaction.EmitEvent(&types.Event{
				Type:     types.EventFailed,
				Message:  "step execution failed",
				Duration: time.Since(stepStartedAt),
				Error:    err,
				Subject:  stepSubject,
			})
			return false, false, err
		}

		ticks.Count++
		ticks.LastExecution = time.Now().UnixMilli()

		if !transaction.ClientInputsBuilder.IsNewEmpty() {
			transaction.State.PushTracking(transaction.Journey.ID, transaction.CurrentStepID)
			transaction.EmitEvent(&types.Event{
				Type:     types.EventSuspended,
				Message:  "journey suspended waiting for client input",
				Duration: time.Since(stepStartedAt),
				Subject: types.EventSubject{
					Type: "journey", ID: transaction.Journey.ID, Name: transaction.Journey.Name,
				},
				Attrs: map[string]any{
					"realm":         transaction.Journey.Realm,
					"client_inputs": len(transaction.ClientInputsBuilder.GetNewInputs()),
				},
			})
			return true, false, nil
		}

		transaction.EmitEvent(&types.Event{
			Type:     types.EventFinished,
			Message:  "step execution finished",
			Duration: time.Since(stepStartedAt),
			Subject:  stepSubject,
			Attrs:    map[string]any{"outcome": outcome},
		})

		if step.EndJourney() {
			completion, reportsCompletion := step.(types.JourneyCompletion)
			failed := reportsCompletion && !completion.JourneySucceeded()
			if transaction.State.ExistsTracking() {
				return false, false, nil
			}
			return false, failed, nil
		}

		if outcome == "" {
			jm.resetClientInputs(transaction)
			return false, false, nil
		}
		transaction.ClientInputsBuilder.ClearRequestsForStep(transaction.CurrentStepID)

		nextStepID, err := stepConfig.GetOutcomeID(outcome)
		if err != nil {
			return false, false, ErrJourneyInvalidOutcome
		}
		transaction.CurrentStepID = nextStepID
	}
}

func journeyEventAttrs(transaction *types.JourneyTransaction) map[string]any {
	attrs := map[string]any{}
	if transaction == nil {
		return attrs
	}
	if transaction.Journey != nil {
		attrs["journey"] = map[string]any{
			"id":    transaction.Journey.ID,
			"name":  transaction.Journey.Name,
			"type":  transaction.Journey.JourneyType,
			"realm": transaction.Journey.Realm,
		}
	}
	if transaction.State != nil {
		if realm := transaction.State.GetRealm(); realm != "" {
			journeyAttrs := nestedEventAttrs(attrs, "journey")
			journeyAttrs["realm"] = realm
		}
		attrs["tracking_depth"] = len(transaction.State.TrackingsID)
	}
	if transaction.Payload != nil {
		attrs["resume"] = transaction.Payload.ResumeID != "" || transaction.Payload.TokenPresent()
	}
	return attrs
}

func nestedEventAttrs(attrs map[string]any, key string) map[string]any {
	if attrs == nil {
		return map[string]any{}
	}
	if nested, ok := attrs[key].(map[string]any); ok {
		return nested
	}
	nested := map[string]any{}
	attrs[key] = nested
	return nested
}

func (jm *journeyManager) pauseJourney(transaction *types.JourneyTransaction) (*types.JourneyPayloadReq, *types.JourneyState, error) {
	extension := consumeExpirationExtension(transaction.State.GetCtx())
	if err := jm.serializeJourneyContext(transaction.State); err != nil {
		return nil, nil, err
	}

	response := &types.JourneyPayloadReq{ClientInputs: transaction.ClientInputsBuilder.GetNewInputs()}
	if transaction.State.GetTempCtx().IsDefined(env.GetSuspendJourneyKey()) {
		resumeID, _ := transaction.State.GetTempCtx().Get(env.GetSuspendJourneyKey() + ".resume_id").AsString()
		suspendTTL := time.Duration(transaction.State.GetTempCtx().Get(env.GetSuspendJourneyKey()+".exp").AsIntOr(60)) * time.Second
		transaction.State.SetID(resumeID)
		transaction.State.ClearState()
		if !jm.states.Store(transaction.State, suspendTTL+extension) {
			return nil, nil, ErrJourneyStateStore
		}
		return response, transaction.State, nil
	}

	journeyToken, err := jm.tokens.Sign(transaction.State)
	if err != nil {
		return nil, nil, err
	}
	response.Jwt = string(journeyToken)
	transaction.State.ClearState()
	if !jm.states.Store(transaction.State, transaction.State.Exp+extension) {
		return nil, nil, ErrJourneyStateStore
	}
	return response, nil, nil
}

func (jm *journeyManager) serializeJourneyContext(state *types.JourneyState) error {
	ctx, err := state.GetCtx().AsMap()
	if err != nil {
		return ErrJourneyFailure
	}
	state.Ctx = ctx

	encryptedCtx, err := state.GetEncryptedCtx().AsMap()
	if err != nil || len(encryptedCtx) == 0 {
		return nil
	}
	state.EncryptedCtx, err = types.EncryptCtx(encryptedCtx, jm.encryptKey)
	if err != nil {
		return ErrJourneyFailure
	}
	return nil
}

func (jm *journeyManager) resetClientInputs(transaction *types.JourneyTransaction) {
	ctx := transaction.ClientInputsBuilder.GetCtxManager()
	ctx.Delete(env.GetClientInputsKey())
	transaction.ClientInputsBuilder = inputs.NewClientInputBuilder([]*inputs.ClientInput{}, ctx, transaction.CacheManager)
}
