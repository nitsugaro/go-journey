package journey_test

import (
	"errors"
	"testing"
	"time"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
)

type listenerTestTokens struct{}

func (listenerTestTokens) Validate(string) (*types.JourneyState, error) {
	return nil, errors.New("listener test token validation is not implemented")
}

func (listenerTestTokens) Sign(*types.JourneyState) ([]byte, error) {
	return []byte("listener-test-token"), nil
}

func TestJourneySuccessListenerRunsAsyncOnTerminalSuccess(t *testing.T) {
	const journeyID = "00000000-0000-0000-0000-000000000901"
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: journeyID},
		Name:        "listener-success",
		Active:      true,
		DefaultExp:  1,
		StartStepID: "success",
		Steps: map[string]*types.Step{
			"success": {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		Tokens:         listenerTestTokens{},
	})
	events := make(chan *types.JourneyExecutionEvent, 1)
	manager.OnJourneySuccess(func(event *types.JourneyExecutionEvent) {
		events <- event
	})

	response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journeyID}})
	if err != nil || response != nil || state == nil {
		t.Fatalf("execution = response %v, state %v, error %v", response, state, err)
	}
	select {
	case event := <-events:
		if event.Status != types.JourneyExecutionSucceeded || event.Journey == nil || event.Journey.ID != journeyID || event.State == nil || event.Error != nil {
			t.Fatalf("success event mismatch: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("success listener was not called")
	}
}

func TestJourneyFailureListenerRunsAsyncOnTerminalFailure(t *testing.T) {
	const journeyID = "00000000-0000-0000-0000-000000000902"
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: journeyID},
		Name:        "listener-failure",
		Active:      true,
		DefaultExp:  1,
		StartStepID: "failure",
		Steps: map[string]*types.Step{
			"failure": {Name: "Failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		Tokens:         listenerTestTokens{},
	})
	events := make(chan *types.JourneyExecutionEvent, 1)
	manager.OnJourneyFailure(func(event *types.JourneyExecutionEvent) {
		events <- event
	})

	response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journeyID}})
	if !errors.Is(err, gojourney.ErrJourneyFailure) || response != nil || state == nil {
		t.Fatalf("execution = response %v, state %v, error %v", response, state, err)
	}
	select {
	case event := <-events:
		if event.Status != types.JourneyExecutionFailed || event.Journey == nil || event.Journey.ID != journeyID || event.State == nil || !errors.Is(event.Error, gojourney.ErrJourneyFailure) {
			t.Fatalf("failure event mismatch: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("failure listener was not called")
	}
}

func TestJourneyListenersDoNotRunOnSuspension(t *testing.T) {
	const (
		journeyID = "00000000-0000-0000-0000-000000000903"
		formID    = "00000000-0000-0000-0000-000000000904"
		successID = "00000000-0000-0000-0000-000000000905"
	)
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: journeyID},
		Name:        "listener-suspended",
		Active:      true,
		DefaultExp:  1,
		StartStepID: formID,
		Steps: map[string]*types.Step{
			formID: {
				Name: "Collect input", StepType: types.FormStep,
				Config: map[string]any{
					"context": "ctx",
					"inputs": []any{map[string]any{
						"id": "name", "external_id": "name", "label": "Name", "type": "string", "required": true,
					}},
					"outcome": map[string]any{"true": successID},
				},
			},
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		Tokens:         listenerTestTokens{},
	})
	events := make(chan *types.JourneyExecutionEvent, 1)
	manager.OnJourneySuccess(func(event *types.JourneyExecutionEvent) { events <- event })
	manager.OnJourneyFailure(func(event *types.JourneyExecutionEvent) { events <- event })

	response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journeyID}})
	if err != nil || response == nil || state != nil {
		t.Fatalf("suspension = response %v, state %v, error %v", response, state, err)
	}
	select {
	case event := <-events:
		t.Fatalf("listener should not run on suspension: %+v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestJourneyListenerPanicIsolated(t *testing.T) {
	const journeyID = "00000000-0000-0000-0000-000000000906"
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	journey := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: journeyID},
		Name:        "listener-panic",
		Active:      true,
		DefaultExp:  1,
		StartStepID: "success",
		Steps: map[string]*types.Step{
			"success": {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
	})
	called := make(chan struct{}, 1)
	manager.OnJourneySuccess(func(*types.JourneyExecutionEvent) {
		panic("listener panic")
	})
	manager.OnJourneySuccess(func(*types.JourneyExecutionEvent) {
		called <- struct{}{}
	})

	if _, _, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journeyID}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("healthy listener was blocked by a panicking listener")
	}
}
