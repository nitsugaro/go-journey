package journey_test

import (
	"bytes"
	"errors"
	"testing"

	gojourney "github.com/nitsugaro/go-journey"
	"github.com/nitsugaro/go-journey/steps"
	"github.com/nitsugaro/go-journey/types"
	"github.com/nitsugaro/go-nstore"
	goutils "github.com/nitsugaro/go-utils/v2"
)

type customSuccessStep struct{}

func (*customSuccessStep) GetStepType() string { return types.SuccessStep }
func (*customSuccessStep) EndJourney() bool    { return true }
func (*customSuccessStep) VerifyConfig(string, goutils.TreeMapImpl) error {
	return nil
}

func TestSubJourneyTerminalStepReturnsToParentOutcome(t *testing.T) {
	const (
		parentID = "00000000-0000-0000-0000-000000000301"
		childID  = "00000000-0000-0000-0000-000000000302"
		subID    = "00000000-0000-0000-0000-000000000303"
		success  = "00000000-0000-0000-0000-000000000304"
		failure  = "00000000-0000-0000-0000-000000000305"
	)

	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	child := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: childID}, Name: "child-success",
		Active: true, DefaultExp: 1, StartStepID: success,
		Steps: map[string]*types.Step{
			success: {Name: "Child success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	parent := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: parentID}, Name: "parent-routes-child-success",
		Active: true, DefaultExp: 1, StartStepID: subID,
		Steps: map[string]*types.Step{
			subID: {
				Name: "Run child", StepType: types.SubJourneyStep,
				Config: map[string]any{
					"journey_id": childID,
					"outcome":    map[string]any{"true": failure, "false": success},
				},
			},
			success: {Name: "Parent success", StepType: types.SuccessStep, Config: map[string]any{}},
			failure: {Name: "Parent failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(child); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(parent); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
	})

	_, _, err = manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: parentID}})
	if !errors.Is(err, gojourney.ErrJourneyFailure) {
		t.Fatalf("child success should route to parent failure, err=%v", err)
	}
}

func TestSubJourneyFailureTerminalStepReturnsFalseOutcome(t *testing.T) {
	const (
		parentID = "00000000-0000-0000-0000-000000000311"
		childID  = "00000000-0000-0000-0000-000000000312"
		subID    = "00000000-0000-0000-0000-000000000313"
		success  = "00000000-0000-0000-0000-000000000314"
		failure  = "00000000-0000-0000-0000-000000000315"
	)

	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	child := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: childID}, Name: "child-failure",
		Active: true, DefaultExp: 1, StartStepID: failure,
		Steps: map[string]*types.Step{
			failure: {Name: "Child failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	parent := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: parentID}, Name: "parent-routes-child-failure",
		Active: true, DefaultExp: 1, StartStepID: subID,
		Steps: map[string]*types.Step{
			subID: {
				Name: "Run child", StepType: types.SubJourneyStep,
				Config: map[string]any{
					"journey_id": childID,
					"outcome":    map[string]any{"true": failure, "false": success},
				},
			},
			success: {Name: "Parent success", StepType: types.SuccessStep, Config: map[string]any{}},
			failure: {Name: "Parent failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	if err := storage.Save(child); err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(parent); err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
	})

	response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: parentID}})
	if err != nil || response != nil || state == nil {
		t.Fatalf("child failure should route to parent success: response=%v state=%v err=%v", response, state, err)
	}
}
func (*customSuccessStep) Execute(*types.JourneyTransaction, goutils.TreeMapImpl) (string, error) {
	return "custom", nil
}

func TestCustomRegistryExtendsDefaultsAndCanOverride(t *testing.T) {
	customSuccess := &customSuccessStep{}
	registry := types.NewStepRegistry()
	registry.AddStep(customSuccess, nil)

	journey := &types.JourneyConfiguration{
		Metadata:    &nstore.Metadata{ID: "custom-storage"},
		Name:        "custom-storage",
		Active:      true,
		DefaultExp:  1,
		StartStepID: "success",
		Steps: map[string]*types.Step{
			"success": {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
		},
	}
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), registry)
	if err != nil {
		t.Fatal(err)
	}
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		Steps:          registry,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
	})
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}

	if registry.GetStep(types.SuccessStep) != customSuccess {
		t.Fatal("custom implementation did not override the default")
	}
	if registry.GetStep(types.FailureStep) == nil || registry.GetStep(types.HttpRequestStep) == nil {
		t.Fatal("custom registry lost default steps")
	}
	mergedSchema, merged := registry.GetSchemas().GetSchema(types.HttpRequestStep)
	defaultSchema, exists := steps.GetDefaultStepRegistry().GetSchemas().GetSchema(types.HttpRequestStep)
	if !merged || !exists || !bytes.Equal(mergedSchema, defaultSchema) {
		t.Fatal("default step schema metadata was not preserved while merging registries")
	}
	response, state, err := manager.InvokeJourney(&types.JourneyExecute{
		Payload: &types.JourneyPayloadReq{JourneyID: journey.ID},
	})
	if err != nil || response != nil || state == nil {
		t.Fatalf("custom configuration storage execution = response %v, state %v, error %v", response, state, err)
	}
}

func TestCustomPlaceholderResolverFromManagerConfig(t *testing.T) {
	const (
		transformID = "00000000-0000-0000-0000-000000000201"
		successID   = "00000000-0000-0000-0000-000000000202"
		failureID   = "00000000-0000-0000-0000-000000000203"
	)
	journey := &types.JourneyConfiguration{
		Metadata: &nstore.Metadata{ID: "custom-placeholder"}, Name: "custom-placeholder",
		Active: true, DefaultExp: 1, StartStepID: transformID,
		Steps: map[string]*types.Step{
			transformID: {
				Name: "Resolve tenant", StepType: types.TransformStep,
				Config: map[string]any{
					"target_context": "ctx",
					"fields":         []any{map[string]any{"target": "tenantName", "value": "${tenant.profile.name}"}},
					"outcome":        map[string]any{"true": successID, "error": failureID},
				},
			},
			successID: {Name: "Success", StepType: types.SuccessStep, Config: map[string]any{}},
			failureID: {Name: "Failure", StepType: types.FailureStep, Config: map[string]any{}},
		},
	}
	storage, err := gojourney.NewJourneyStorage(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Save(journey); err != nil {
		t.Fatal(err)
	}
	calledWith := ""
	manager := gojourney.NewManager(&gojourney.JourneyManagerConfig{
		JourneyStorage: storage,
		EncryptKey:     []byte("0123456789abcdef0123456789abcdef"),
		PlaceholderResolvers: map[string]types.PlaceholderResolver{
			"tenant": func(path string) (any, error) {
				calledWith = path
				return "Acme", nil
			},
		},
	})

	response, state, err := manager.InvokeJourney(&types.JourneyExecute{Payload: &types.JourneyPayloadReq{JourneyID: journey.ID}})
	if err != nil || response != nil || state == nil {
		t.Fatalf("execution = response %v, state %v, error %v", response, state, err)
	}
	if calledWith != "profile.name" {
		t.Fatalf("resolver path = %q", calledWith)
	}
	if got := state.GetCtx().Get("tenantName").AsStringOr(""); got != "Acme" {
		t.Fatalf("resolved tenant = %q", got)
	}
}
